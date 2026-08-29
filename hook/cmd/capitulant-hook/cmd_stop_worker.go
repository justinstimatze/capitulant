package main

import (
	"encoding/json"
	"os"

	"github.com/justinstimatze/capitulant/hook/internal/classify"
	"github.com/justinstimatze/capitulant/hook/internal/notify"
	"github.com/justinstimatze/capitulant/hook/internal/substrate"
)

// cmdStopWorker is spawnStopWorker's detached child. Never invoked by
// Claude Code or by hand -- only ever by cmdStop's own exec.Command
// call, with a payload file path as its one argument. Reads and deletes
// that file immediately, then does exactly what cmdStop used to do
// inline before this file existed: classify, append to substrate,
// notify on YES. Also the inline fallback path if spawning the detached
// process itself fails, via runStopWorker directly -- see cmd_stop.go.
func cmdStopWorker(args []string) {
	if len(args) != 1 {
		logf("stop-worker", "expected exactly one payload path argument, got %d", len(args))
		return
	}
	payloadPath := args[0]

	data, err := os.ReadFile(payloadPath)
	os.Remove(payloadPath)
	if err != nil {
		logf("stop-worker", "reading payload %s: %s", payloadPath, err)
		return
	}
	var p stopWorkerPayload
	if err := json.Unmarshal(data, &p); err != nil {
		logf("stop-worker", "bad payload JSON: %s", err)
		return
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		logf("stop-worker", "ANTHROPIC_API_KEY unset, skipping classification")
		return
	}

	runStopWorker("stop-worker", p.SessionID, p.Text, apiKey)
}

// runStopWorker is the actual classify+substrate+notify work, shared by
// the detached worker (cmdStopWorker) and cmdStop's inline fallback if
// the detach itself couldn't be started. tag names which path is
// running, for the log line only.
func runStopWorker(tag, sessionID, text, apiKey string) {
	result, err := classify.Run(text, apiKey)
	if err != nil {
		logf(tag, "classify error: %s", err)
		return
	}
	if result.Verdict == "" {
		logf(tag, "unparseable classifier response")
		return
	}

	if err := substrate.Append(substrate.NewRecord(sessionID, text, result.Verdict, result.Category, result.Notes)); err != nil {
		logf(tag, "substrate append failed: %s", err)
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
		logf(tag, "notify failed: %s", err)
	}
}
