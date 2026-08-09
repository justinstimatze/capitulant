// Package notify fires one desktop notification. The immediate,
// local-only half of the two-channel design from the hybrid-loops pass:
// this reaches a human at the machine right now; the inject hook
// (UserPromptSubmit, next Claude Code turn) is the fallback for when
// they're not.
package notify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const timeout = 10 * time.Second

// Send delivers title/body via notify-send. Linux-only for now -- this
// project runs on one host; darwin/osascript support can follow the
// same pattern nowcast uses if this ever needs to travel.
func Send(title, body string) error {
	p, err := exec.LookPath("notify-send")
	if err != nil {
		return fmt.Errorf("notify: notify-send not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// "--" terminates option parsing before the title/body, which are
	// not under this program's control (they come from a classifier's
	// output) -- without it, a title starting with "-" would be parsed
	// as a notify-send flag instead of displayed text.
	c := exec.CommandContext(ctx, p, "--app-name=capitulant", "--urgency=critical",
		"--icon=dialog-warning", "--", title, body)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("notify: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
