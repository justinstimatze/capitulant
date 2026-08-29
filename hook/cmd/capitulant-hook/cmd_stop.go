package main

import (
	"os"

	"github.com/justinstimatze/capitulant/hook/internal/classify"
	"github.com/justinstimatze/capitulant/hook/internal/filter"
	"github.com/justinstimatze/capitulant/hook/internal/hookio"
	"github.com/justinstimatze/capitulant/hook/internal/notify"
	"github.com/justinstimatze/capitulant/hook/internal/substrate"
	"github.com/justinstimatze/capitulant/hook/internal/transcript"
)

// cmdStop is the Stop hook: read the turn that just ended, run it through
// the free local pre-filter (internal/filter), and only pay for an API
// call if that matches. Always logs the result to the substrate --
// verdict NO and UNCLEAR too, not just YES, since a log of only hits has
// no denominator for a hit-rate calibration pass later.
func cmdStop() {
	in := hookio.Read(os.Stdin)
	if in.TranscriptPath == "" {
		return
	}

	text := transcript.LastAssistantText(in.TranscriptPath)
	if text == "" {
		return
	}

	if !filter.Matches(text) {
		return
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		logf("stop", "pre-filter matched but ANTHROPIC_API_KEY unset, skipping classification: %.80s", text)
		return
	}

	result, err := classify.Run(text, apiKey)
	if err != nil {
		logf("stop", "classify error: %s", err)
		return
	}
	if result.Verdict == "" {
		logf("stop", "unparseable classifier response")
		return
	}

	if err := substrate.Append(substrate.NewRecord(in.SessionID, text, result.Verdict, result.Category, result.Notes)); err != nil {
		logf("stop", "substrate append failed: %s", err)
	}

	if result.Verdict != "YES" {
		return
	}

	title := "capitulant: possible scope-violation rationalization"
	body := result.Category
	if body == "" {
		body = "(category unparsed)"
	}
	if err := notify.Send(title, body); err != nil {
		logf("stop", "notify failed: %s", err)
	}
}
