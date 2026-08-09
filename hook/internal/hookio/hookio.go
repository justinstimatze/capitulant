// Package hookio reads the JSON Claude Code sends a hook on stdin.
package hookio

import (
	"encoding/json"
	"io"
)

// Input is the subset of Claude Code's hook payload this project needs.
// Field names match Claude Code's own JSON exactly (session_id,
// transcript_path, hook_event_name) -- confirmed against a working hook
// implementation (nowcast) rather than guessed from docs.
type Input struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Event          string `json:"hook_event_name"`
}

// Read parses stdin, capped at 1MB -- a malformed or hostile payload
// should not be able to make a hook read unbounded input.
func Read(r io.Reader) Input {
	var in Input
	data, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return in
	}
	_ = json.Unmarshal(data, &in)
	return in
}
