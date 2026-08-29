// Command capitulant-hook is the Stop + UserPromptSubmit hook pair that
// makes the classifier in eval/run_judge.py a live, running loop instead
// of a one-off score against a static eval set. See hook/README.md and
// LIT_REVIEW.md's "Roadmap" for the design.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "stop":
		guard("stop", cmdStop)
	case "inject":
		guard("inject", cmdInject)
	case "install":
		cmdInstall(os.Args[2:])
	case "uninstall":
		cmdUninstall(os.Args[2:])
	case "bench":
		cmdBench(os.Args[2:])
	case "version":
		fmt.Println("capitulant-hook (unversioned prototype)")
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `capitulant-hook -- live Stop/UserPromptSubmit hooks for the capitulant classifier

usage:
  capitulant-hook stop               Stop hook: classify the turn that just ended (not run by hand)
  capitulant-hook inject             UserPromptSubmit hook: surface unacknowledged findings (not run by hand)
  capitulant-hook install [--write]  print or apply the project-local settings.json wiring
  capitulant-hook uninstall          remove capitulant's hooks from ./.claude/settings.json
  capitulant-hook bench -out FILE    score classify.Run against eval/seed_set.jsonl, write results to FILE
  capitulant-hook version            print version

Requires ANTHROPIC_API_KEY in the environment for "stop" to actually classify anything;
without it, "stop" logs and exits 0 rather than blocking the turn. "bench" also requires
it and exits 1 immediately if unset, since every case it scores is a real API call.`)
}

// guard wraps a hook entry point so a panic is logged, never surfaced as
// a nonzero exit. A hook that crashes with a nonzero exit can interfere
// with the user's turn; one that silently no-ops only loses a finding.
func guard(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			logf(name, "PANIC %v", r)
		}
		os.Exit(0)
	}()
	fn()
}
