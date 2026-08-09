// Package substrate is the accumulating typed record of every lens call
// -- the piece capitulant didn't have before this hook: CODEBOOK.md is
// substrate-as-vocabulary (a fixed taxonomy), this is substrate-as-record
// (what actually happened, over time). Logs every verdict, not just
// positives -- a log of only hits has no denominator, and no calibration
// pass (comparing raw flag rate to human-verified hit rate, the way the
// one real-pool pass in LIT_REVIEW.md did) is possible without one.
package substrate

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const schemaVersion = 1

// Record is one lens call, win or lose. Field set matches the schema
// sketched in the hybrid-loops design pass: timestamp, session, excerpt,
// verdict, category, model, schema version, notes.
type Record struct {
	Timestamp     time.Time `json:"timestamp"`
	SessionID     string    `json:"session_id"`
	Excerpt       string    `json:"excerpt"`
	Verdict       string    `json:"verdict"`
	Category      string    `json:"category,omitempty"`
	Model         string    `json:"model"`
	SchemaVersion int       `json:"schema_version"`
	Notes         string    `json:"notes"`
	Acknowledged  bool      `json:"acknowledged"`
}

// Dir returns the state directory, creating it. Follows the same
// override chain as nowcast's state dir: an explicit env var, then XDG,
// then a dotted fallback under home.
func Dir() (string, error) {
	dir := os.Getenv("CAPITULANT_STATE_DIR")
	if dir == "" {
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			dir = filepath.Join(xdg, "capitulant")
		} else if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, ".local", "state", "capitulant")
		} else {
			return "", err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "findings.jsonl"), nil
}

// Append writes one record. NewRecord sets SchemaVersion and Timestamp;
// callers fill in the rest.
func Append(r Record) error {
	return withLock(func() error {
		p, err := path()
		if err != nil {
			return err
		}
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		return enc.Encode(r)
	})
}

// NewRecord fills in the fields that don't vary per call.
func NewRecord(sessionID, excerpt, verdict, category, notes string) Record {
	return Record{
		Timestamp:     time.Now().UTC(),
		SessionID:     sessionID,
		Excerpt:       excerpt,
		Verdict:       verdict,
		Category:      category,
		Model:         "gpt-oss-20b",
		SchemaVersion: schemaVersion,
		Notes:         notes,
		Acknowledged:  false,
	}
}

// ShowUnacknowledged finds every YES-verdict record not yet surfaced to
// a human, passes them to show, and -- only if show returns nil --
// atomically marks exactly those records (and no others) acknowledged,
// all under one held lock. Returns the count shown.
//
// The lock is held across the call to show, not just the disk rewrite.
// That's deliberate, not an accident of implementation convenience: it
// closes two separate bugs an earlier version of this file had.
//
//  1. A lock-free rewrite could lose a concurrent Append from a second
//     Claude Code session in the same project.
//  2. Acknowledging first and printing after (the original order) means
//     a process killed between the two steps marks a finding seen that
//     was never actually displayed -- the disk write and the "a human
//     saw this" fact must happen as one unit, or the acknowledgment is
//     a claim the code can't back up. show is expected to be fast (a
//     stdout print); holding the lock across it is the cost of that
//     guarantee actually meaning what it says.
func ShowUnacknowledged(show func([]Record) error) (int, error) {
	shown := 0
	err := withLock(func() error {
		p, err := path()
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}

		var all []Record
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			var r Record
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				continue
			}
			all = append(all, r)
		}
		f.Close()

		var pending []Record
		for _, r := range all {
			if r.Verdict == "YES" && !r.Acknowledged {
				pending = append(pending, r)
			}
		}
		if len(pending) == 0 {
			return nil
		}

		if err := show(pending); err != nil {
			return err
		}
		shown = len(pending)

		for i := range all {
			if all[i].Verdict == "YES" && !all[i].Acknowledged {
				all[i].Acknowledged = true
			}
		}

		tmp := p + ".tmp"
		w, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(w)
		for _, r := range all {
			if err := enc.Encode(r); err != nil {
				w.Close()
				return err
			}
		}
		if err := w.Close(); err != nil {
			return err
		}
		return os.Rename(tmp, p)
	})
	return shown, err
}
