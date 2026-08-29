package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"syscall"

	"github.com/justinstimatze/capitulant/hook/internal/filter"
	"github.com/justinstimatze/capitulant/hook/internal/hookio"
	"github.com/justinstimatze/capitulant/hook/internal/substrate"
	"github.com/justinstimatze/capitulant/hook/internal/transcript"
)

// cmdStop is the Stop hook: read the turn that just ended, run it through
// the free local pre-filter (internal/filter), and only pay for an API
// call if that matches. The actual classify.Run call is handed off to a
// detached "stop-worker" subprocess (cmd_stop_worker.go) rather than run
// inline -- classify.Run's HTTP round-trip (~1s typical, up to the 60s
// client timeout worst case) would otherwise block Claude Code from
// considering this hook done, and with it the next turn, on every single
// matched turn. If spawning the detached worker itself fails, falls back
// to classifying inline rather than silently dropping the finding.
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

	if err := spawnStopWorker(in.SessionID, text); err != nil {
		logf("stop", "detach failed (%s), classifying inline instead", err)
		runStopWorker("stop-inline-fallback", in.SessionID, text, apiKey)
	}
}

type stopWorkerPayload struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

// spawnStopWorker hands the classify+substrate+notify work to a detached
// child process and returns as soon as it's launched, without waiting
// for it to finish. The payload (session id + excerpt text) goes through
// a file rather than argv or a pipe -- argv has length/escaping limits
// for arbitrary transcript text, and a pipe's write end would need to
// outlive this process's exit, which os/exec doesn't support cleanly.
// The file lives in substrate.Dir() (0700, same place findings.jsonl
// lives) rather than the system temp dir, since it briefly holds the
// same excerpt text the substrate already stores -- no reason to let it
// pass through a world-readable location even for a moment.
//
// The child is started with Setsid so it gets its own session and
// process group: without that, it would inherit the hook process's
// process group, and a Ctrl-C in the terminal that sends SIGINT to the
// whole foreground process group would kill the worker mid-classify,
// losing the finding for no reason related to the classification itself.
// Stdin/Stdout/Stderr are left nil (-> /dev/null): if the child inherited
// this process's stdout/stderr, which are Claude Code's own hook-result
// pipes, Claude Code could block waiting for those pipes to reach EOF
// until the detached worker eventually exits -- exactly the blocking
// this change exists to remove.
//
// Not calling Wait() on the child does not leak a zombie: a zombie is a
// child that has exited but whose parent hasn't reaped it while the
// parent is still running. Once this process exits (main's guard() calls
// os.Exit(0) right after cmdStop returns), the worker is reparented and
// reaped normally when it finishes.
func spawnStopWorker(sessionID, text string) error {
	dir, err := substrate.Dir()
	if err != nil {
		return err
	}
	payload, err := os.CreateTemp(dir, "stop-worker-*.json")
	if err != nil {
		return err
	}
	defer payload.Close()
	if err := json.NewEncoder(payload).Encode(stopWorkerPayload{SessionID: sessionID, Text: text}); err != nil {
		os.Remove(payload.Name())
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		os.Remove(payload.Name())
		return err
	}

	cmd := exec.Command(exe, "stop-worker", payload.Name())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		os.Remove(payload.Name())
		return err
	}
	return nil
}
