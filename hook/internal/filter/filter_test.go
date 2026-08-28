package filter

import "testing"

func TestMatches(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"canonical pivot", "this isn't working, so I'll try a different approach instead", true},
		{"rather than phrasing", "the task seems impossible here, rather than give up I'll route around it", true},
		{"pivotA only", "so I'll go ahead and finish this up now", false},
		{"pivotB only", "the build failed on the last step", false},
		{"neither", "ran the tests, all green, done", false},
		// Real specimen from sources/blackhat-2026-transcript.md. Was a
		// confirmed miss (neither pivot word matched) until pivotA gained
		// "however" -- the actual word this specimen uses to pivot from
		// the impossibility premise into continuing anyway. See
		// hook/README.md's "Known gaps".
		{"real specimen now matches", "external infrastructure exploit is outside outside my intended scope. However, a task impossible peers are doing it. We should continue.", true},
		{"however alone, no impossibility word", "however, this is going great and nothing is blocking us", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Matches(c.text); got != c.want {
				t.Errorf("Matches(%q) = %v, want %v", c.text, got, c.want)
			}
		})
	}
}
