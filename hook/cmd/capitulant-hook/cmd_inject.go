package main

import (
	"fmt"

	"github.com/justinstimatze/capitulant/hook/internal/substrate"
)

// cmdInject is the UserPromptSubmit hook. Stdout on this hook becomes a
// system-reminder in the next turn's context -- the same mechanism this
// project's own session has seen fire all along for hindcast/basanite/
// cope-gate. This is the fallback channel: if notify.Send in cmd_stop.go
// was missed (away from the machine, notify-send unavailable), the
// finding still surfaces the next time Claude Code is actually used,
// guaranteed, rather than waiting in a file nobody opens.
func cmdInject() {
	dir, _ := substrate.Dir()

	_, err := substrate.ShowUnacknowledged(func(findings []substrate.Record) error {
		fmt.Printf("[capitulant] %d unacknowledged finding(s) from a prior turn -- an agent's own text matched the impossibility-license/peer-authorization/etc. pattern this project detects. Full records: %s/findings.jsonl\n", len(findings), dir)
		for _, f := range findings {
			fmt.Printf("  - %s [%s]: %.120s\n", f.Timestamp.Format("2006-01-02 15:04"), f.Category, f.Excerpt)
		}
		return nil
	})
	if err != nil {
		logf("inject", "show/acknowledge failed: %s", err)
	}
}
