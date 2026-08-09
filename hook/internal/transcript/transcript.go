// Package transcript reads the text of the most recent assistant turn
// from a Claude Code session's JSONL transcript.
package transcript

import (
	"bufio"
	"encoding/json"
	"os"
)

type message struct {
	Content json.RawMessage `json:"content"`
}

type entry struct {
	Type    string  `json:"type"`
	Message message `json:"message"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// LastAssistantText returns the concatenated text blocks of the last
// `type: assistant` entry in path, or "" if none is found or the file
// can't be read. Non-text blocks (tool_use, tool_result, thinking -- see
// LIT_REVIEW.md on why thinking is empty in practice) are skipped.
//
// Reads the whole file rather than seeking from the end: transcripts are
// per-session and small enough (low thousands of lines at most) that a
// full scan is simpler and safer than reconstructing JSONL boundaries
// from an arbitrary byte offset.
func LastAssistantText(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var last string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for sc.Scan() {
		var e entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if e.Type != "assistant" {
			continue
		}
		if text := extractText(e.Message.Content); text != "" {
			last = text
		}
	}
	return last
}

// extractText handles both shapes Claude Code's transcript format uses
// for message.content: a plain string, or an array of typed blocks.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	out := ""
	for _, b := range blocks {
		if b.Type == "text" {
			out += b.Text + "\n"
		}
	}
	return out
}
