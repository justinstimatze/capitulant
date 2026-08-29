package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/justinstimatze/capitulant/hook/internal/substrate"
)

// TestStopInjectEndToEnd runs the actual compiled capitulant-hook binary
// against a realistic, full-shaped session transcript (testdata/realistic-
// session.jsonl) -- not a scored string from eval/seed_set.jsonl. That eval
// set only ever exercises classify.Run against a hand-picked excerpt; it
// never touches transcript parsing, the pre-filter against real formatting,
// the substrate's on-disk side effect, or the inject readback, because a
// seed-set case is text, never a transcript file going through the binary
// the way Claude Code actually invokes it (stdin JSON in, stdout/exit code
// out, one process per hook event). This test is that missing path, run
// once, start to finish, instead of scoring pieces of it in isolation. Uses
// classify.Run's Anthropic Messages API backend, not eval/run_judge.py's
// gpt-oss-20b path -- the two aren't wired to the same backend.
//
// Skips without ANTHROPIC_API_KEY set: this makes a real classify.Run
// network call against Anthropic's Messages API, and should never run
// silently as part of a default `go test ./...` and cost an API call
// nobody asked for. Run explicitly with ANTHROPIC_API_KEY=sk-ant-... go
// test ./cmd/... -run TestStopInjectEndToEnd -v to actually exercise it.
func TestStopInjectEndToEnd(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set -- this test makes a real classify.Run API call, skipping rather than running it silently")
	}

	bin := buildHook(t)
	stateDir := t.TempDir()
	env := cleanEnv(apiKey, stateDir)

	transcriptPath, err := filepath.Abs(filepath.Join("testdata", "realistic-session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	const sessionID = "capitulant-integration-test"

	// Phase 1: Stop hook, exactly as Claude Code invokes it -- a JSON
	// payload on stdin naming the transcript, nothing else.
	stopIn, err := json.Marshal(map[string]string{
		"session_id":      sessionID,
		"transcript_path": transcriptPath,
		"hook_event_name": "Stop",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := run(t, bin, env, stopIn, "stop"); err != nil {
		t.Fatalf("capitulant-hook stop exited nonzero: %v\nstderr: %s", err, stderr)
	}

	// stop hands classify.Run off to a detached stop-worker subprocess
	// and returns before it finishes (see cmd_stop.go) -- that's the
	// whole point, but it means the substrate record isn't necessarily
	// there the instant stop's own process exits. Poll for it instead of
	// asserting immediately; classify.Run's own http.Client has a 60s
	// timeout, so 20s is a real bound, not an arbitrary one -- typical
	// latency measured elsewhere this project is ~1-2s.
	records := waitForSubstrateRecords(t, stateDir, 1, 20*time.Second)
	if len(records) != 1 {
		t.Fatalf("want exactly one substrate record after one stop invocation against a filter-matching transcript, got %d: %+v", len(records), records)
	}
	rec := records[0]

	if rec.SessionID != sessionID {
		t.Errorf("record session_id = %q, want %q", rec.SessionID, sessionID)
	}
	if !strings.Contains(rec.Excerpt, "impossible without a library upgrade") {
		t.Errorf("record excerpt doesn't contain the fixture's actual last-assistant text -- transcript parsing picked up the wrong turn: %.200q", rec.Excerpt)
	}
	switch rec.Verdict {
	case "YES", "NO", "UNCLEAR":
		// any of the three is a structurally valid classify.Run result;
		// the classifier isn't perfectly deterministic at temperature 0
		// on this serving stack (see LIT_REVIEW.md's "Evaluation"
		// section), so this test doesn't hard-require YES on a
		// license-taking-shaped fixture -- it hard-requires that the
		// whole real pipeline produced a parseable verdict at all.
	default:
		t.Errorf("record verdict = %q, want one of YES/NO/UNCLEAR", rec.Verdict)
	}
	t.Logf("real classify.Run verdict on the realistic fixture: %s (category=%q)", rec.Verdict, rec.Category)

	// Phase 2: inject, as UserPromptSubmit would invoke it on the next
	// turn. Assert against the actual round-trip through disk, not a
	// direct ShowUnacknowledged call.
	injectIn, err := json.Marshal(map[string]string{
		"session_id":      sessionID,
		"hook_event_name": "UserPromptSubmit",
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := run(t, bin, env, injectIn, "inject")
	if err != nil {
		t.Fatalf("capitulant-hook inject exited nonzero: %v\nstderr: %s", err, stderr)
	}

	if rec.Verdict == "YES" {
		if !strings.Contains(stdout, "unacknowledged finding") {
			t.Errorf("verdict was YES but inject's stdout doesn't surface it: %s", stdout)
		}
		if rec.Category != "" && !strings.Contains(stdout, rec.Category) {
			t.Errorf("inject's stdout doesn't name the category %q: %s", rec.Category, stdout)
		}
	} else if stdout != "" {
		t.Errorf("verdict was %s (not YES), want inject to print nothing, got: %s", rec.Verdict, stdout)
	}

	// Phase 3: inject again. Whatever was YES a moment ago must now be
	// acknowledged -- the same property substrate_test.go proves at the
	// function-call level, now proven through the actual binary and an
	// actual second process reading the same on-disk file.
	stdout2, stderr2, err := run(t, bin, env, injectIn, "inject")
	if err != nil {
		t.Fatalf("second capitulant-hook inject exited nonzero: %v\nstderr: %s", err, stderr2)
	}
	if stdout2 != "" {
		t.Errorf("second inject call should show nothing (already acknowledged), got: %s", stdout2)
	}
}

func buildHook(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "capitulant-hook")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, stderr.String())
	}
	return bin
}

// cleanEnv builds a subprocess environment from the current process's own,
// with any pre-existing ANTHROPIC_API_KEY/CAPITULANT_STATE_DIR stripped
// before the intended values are appended -- os/exec's handling of
// duplicate keys in Env isn't something to rely on.
func cleanEnv(apiKey, stateDir string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") || strings.HasPrefix(kv, "CAPITULANT_STATE_DIR=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "ANTHROPIC_API_KEY="+apiKey, "CAPITULANT_STATE_DIR="+stateDir)
}

func run(t *testing.T, bin string, env []string, stdin []byte, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(stdin)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// readSubstrate returns the current records, or nil if findings.jsonl
// doesn't exist yet -- the detached stop-worker creates it lazily on its
// first Append, so "not there yet" is an expected transient state while
// waitForSubstrateRecords is polling, not a fatal condition.
func readSubstrate(t *testing.T, stateDir string) []substrate.Record {
	t.Helper()
	f, err := os.Open(filepath.Join(stateDir, "findings.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("opening findings.jsonl: %v", err)
	}
	defer f.Close()

	var records []substrate.Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r substrate.Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("unmarshaling substrate record: %v\nline: %s", err, sc.Text())
		}
		records = append(records, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}

// waitForSubstrateRecords polls readSubstrate until at least want records
// are present or timeout elapses. Needed because stop-worker runs
// detached from the stop process that spawns it (see cmd_stop.go) -- the
// substrate write happens on its own schedule, not synchronously with
// the parent process's exit.
func waitForSubstrateRecords(t *testing.T, stateDir string, want int, timeout time.Duration) []substrate.Record {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		records := readSubstrate(t, stateDir)
		if len(records) >= want {
			return records
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for >= %d substrate record(s), got %d: %+v", timeout, want, len(records), records)
		}
		time.Sleep(150 * time.Millisecond)
	}
}
