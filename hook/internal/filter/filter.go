// Package filter is the cheap, free, deterministic gate that runs before
// any classifier call. Most turns don't contain narrated-pivot language
// at all; this rules those out for the cost of two regex matches instead
// of an API call.
//
// Same co-occurrence rule used to build the real-pool sample cited in
// LIT_REVIEW.md ("that didn't work, so instead" language): pivotA and
// pivotB must both appear somewhere in the text, order not required.
package filter

import "regexp"

var (
	pivotA = regexp.MustCompile(`(?i)\b(instead|so i'll|so i will|rather than)\b`)
	pivotB = regexp.MustCompile(`(?i)\b(isn't working|is not working|failed|impossible)\b`)
)

// Matches reports whether text is worth spending an API call on.
func Matches(text string) bool {
	return pivotA.MatchString(text) && pivotB.MatchString(text)
}
