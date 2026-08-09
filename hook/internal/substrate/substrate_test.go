package substrate

import (
	"os"
	"sync"
	"testing"
)

func setStateDir(t *testing.T) {
	t.Helper()
	t.Setenv("CAPITULANT_STATE_DIR", t.TempDir())
}

func TestAppendAndShowUnacknowledged(t *testing.T) {
	setStateDir(t)

	if err := Append(NewRecord("s1", "excerpt one", "YES", "impossibility_license", "note")); err != nil {
		t.Fatal(err)
	}
	if err := Append(NewRecord("s1", "excerpt two", "NO", "", "note")); err != nil {
		t.Fatal(err)
	}

	var seen []Record
	n, err := ShowUnacknowledged(func(r []Record) error {
		seen = append(seen, r...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(seen) != 1 {
		t.Fatalf("got %d shown, want exactly the 1 YES record (NO records must not be surfaced)", n)
	}
	if seen[0].Excerpt != "excerpt one" {
		t.Errorf("got excerpt %q", seen[0].Excerpt)
	}

	// Second call: already acknowledged, must show nothing.
	n2, err := ShowUnacknowledged(func(r []Record) error {
		t.Errorf("show called again with %d records; nothing should be unacknowledged", len(r))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Errorf("got %d on second call, want 0", n2)
	}
}

// TestConcurrentAppendDuringShow reproduces the original bug directly:
// a second session's Stop hook appends a new YES record while the first
// session's ShowUnacknowledged is mid-rewrite. Without the lock, the
// rewrite's stale in-memory copy of the file (read before the concurrent
// append landed) would win the rename and silently drop the new record.
func TestConcurrentAppendDuringShow(t *testing.T) {
	setStateDir(t)

	if err := Append(NewRecord("s1", "first", "YES", "impossibility_license", "")); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// The append races the read-modify-write inside ShowUnacknowledged
		// below. withLock serializes them either order; what matters is
		// that neither disappears.
		if err := Append(NewRecord("s2", "second, from a concurrent session", "YES", "peer_authorization", "")); err != nil {
			t.Error(err)
		}
	}()

	n, err := ShowUnacknowledged(func(r []Record) error { return nil })
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	// n is 1 or 2 depending on ordering (both orderings are correct --
	// the concurrent append either lands before or after this read).
	// What must NOT happen: the file ends up with fewer than 2 records
	// total, which is what the unlocked version could silently produce.
	_ = n

	final, err := ShowUnacknowledged(func(r []Record) error {
		// Whatever wasn't caught by the first call must show up here.
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	total := n + final
	if total != 2 {
		t.Fatalf("total records surfaced across both calls = %d, want 2 -- the concurrent append was lost", total)
	}
}

func TestShowFailureDoesNotAcknowledge(t *testing.T) {
	setStateDir(t)

	if err := Append(NewRecord("s1", "excerpt", "YES", "impossibility_license", "")); err != nil {
		t.Fatal(err)
	}

	_, err := ShowUnacknowledged(func(r []Record) error {
		return os.ErrInvalid // simulate a failed print/display
	})
	if err == nil {
		t.Fatal("expected ShowUnacknowledged to propagate show's error")
	}

	// The record must still be unacknowledged -- show failing must not
	// have marked it seen.
	n, err := ShowUnacknowledged(func(r []Record) error {
		if len(r) != 1 {
			t.Errorf("got %d records, want the same 1 that failed to show last time", len(r))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("got %d, want 1 -- a failed show must leave the record unacknowledged", n)
	}
}
