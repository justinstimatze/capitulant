package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLastAssistantText_ArrayContent(t *testing.T) {
	path := write(t, `{"type":"user","message":{"content":"hi"}}
{"type":"assistant","message":{"content":[{"type":"text","text":"first"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","text":""},{"type":"text","text":"second"}]}}
`)
	got := LastAssistantText(path)
	if got != "second\n" {
		t.Errorf("got %q, want the last assistant entry's text block", got)
	}
}

func TestLastAssistantText_StringContent(t *testing.T) {
	path := write(t, `{"type":"assistant","message":{"content":"plain string form"}}
`)
	if got := LastAssistantText(path); got != "plain string form" {
		t.Errorf("got %q", got)
	}
}

func TestLastAssistantText_MissingFile(t *testing.T) {
	if got := LastAssistantText("/nonexistent/path.jsonl"); got != "" {
		t.Errorf("got %q, want empty string for a missing file", got)
	}
}

func TestLastAssistantText_EmptyThinkingBlockDoesNotOverwrite(t *testing.T) {
	// The empirical finding from LIT_REVIEW.md: thinking blocks are
	// consistently empty in local transcripts. A trailing entry with no
	// text block (only an empty thinking block) must not blank out the
	// last real text this turn actually produced.
	path := write(t, `{"type":"assistant","message":{"content":[{"type":"text","text":"real answer"}]}}
{"type":"assistant","message":{"content":[{"type":"thinking","text":""}]}}
`)
	if got := LastAssistantText(path); got != "real answer\n" {
		t.Errorf("got %q, want the last real text block, not blanked by a trailing empty-thinking entry", got)
	}
}
