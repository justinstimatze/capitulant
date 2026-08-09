# capitulant-hook

The live version of the classifier: a Claude Code Stop hook that scores
the turn that just ended, and a UserPromptSubmit hook that surfaces
anything flagged. See `LIT_REVIEW.md`'s "Roadmap" for how this fits the
rest of the project, and the design pass that shaped it.

This is a prototype running against a classifier with no live
calibration data yet — see "Known gaps" below before wiring it into a
project you rely on.

## What it does

1. **`capitulant-hook stop`** (Stop hook): reads the transcript, runs a
   free local regex pre-filter (`internal/filter`) before anything else.
   Most turns don't match and the hook does nothing. A match calls
   gpt-oss-20b (needs `HF_TOKEN`) and appends the result — verdict,
   category, and the model's reasoning — to a local JSONL substrate,
   whether the verdict is YES, NO, or UNCLEAR.
2. On a YES verdict: fires a desktop notification (`notify-send`)
   immediately, *and* marks the finding for the next channel below.
   Two channels, not one — a local notification only reaches someone at
   the machine right now; a missed notification would otherwise vanish.
3. **`capitulant-hook inject`** (UserPromptSubmit hook): on the next
   Claude Code turn in this project, prints any unacknowledged findings
   into context — the same mechanism `hindcast`/`basanite`/`cope-gate`
   already use for their own findings. This is the guaranteed-eventually
   -seen fallback: whichever channel fires, a finding never depends on
   someone remembering to check a file.

Deliberately *not* built: nothing here blocks a tool call or the agent's
turn, and nothing feeds the verdict back into the same agent's context.
Both were considered and rejected — see `LIT_REVIEW.md`'s citations on
why verdict-feedback becomes a persuasion channel rather than a
correction, and why acting on an uncalibrated classifier's raw flag rate
would do more harm than the thing it's trying to catch.

## Install

Project-local only — no global install option. This is untested v0
running an unvalidated classifier; wiring it into every project before
any live calibration exists would be a bigger blast radius than this
prototype has earned.

```
cd hook
go build -o capitulant-hook ./cmd/capitulant-hook
./capitulant-hook install --write
```

Requires `HF_TOKEN` (a Hugging Face Inference Providers key) in the
environment Claude Code runs in for `stop` to actually classify
anything. Without it, `stop` logs and exits 0 rather than blocking the
turn — see `capitulant-hook install` (no `--write`) for what gets wired,
and `capitulant-hook uninstall` to remove it.

## Known gaps

- **The pre-filter has a real, confirmed recall gap.** It only matches
  text containing both an impossibility word (`failed`, `impossible`,
  `isn't working`) and a pivot word (`instead`, `so I'll`, `rather
  than`). The project's own primary specimen — "external infrastructure
  exploit is outside outside my intended scope. However, a task
  impossible peers are doing it. We should continue." — does **not**
  contain a pivot word, and would be silently skipped by this filter
  (confirmed as a test case in `internal/filter/filter_test.go`, not
  discovered after the fact). The filter exists to make most turns free;
  it trades that for missing positives phrased without one of these four
  words. Not fixed here — a looser filter costs more API calls for every
  session it's wired into, which needs its own design pass.
- **No live calibration yet.** The one real-world measurement this
  project has (`LIT_REVIEW.md`'s real-pool pass) found a raw flag rate
  roughly 5-10x higher than the true positive rate. This hook's
  `findings.jsonl` substrate is what makes an actual hit-rate
  measurement possible going forward — logging every verdict, not just
  YES, is specifically so that pass can happen — but it hasn't happened
  yet for *this* hook's own output.
- **`notify-send` only.** No macOS/Windows support. Follows the pattern
  `nowcast`'s `internal/notify` uses if this ever needs to travel.
- **Matched text leaves the machine.** Any turn the pre-filter matches
  has its full text sent to Hugging Face's Inference Providers router
  (routed to Groq, per `eval/run_judge.py`'s own docstring) for
  classification. This is true of the offline `eval/run_judge.py` path
  too, but the hook makes it a standing fact about a project's live
  sessions rather than a one-off eval run — worth knowing before wiring
  this into a project with sensitive content in its transcripts, even
  though it's project-local-only for now.
- **The substrate grows without bound.** Every verdict is appended
  forever; nothing rotates or expires `findings.jsonl`. This is partly
  intentional — a calibration pass needs the full history, not a
  truncated tail — but a project run for a long time will accumulate an
  unbounded local record of its own agent's text with no cleanup path.
  No rotation implemented here.
- **Concurrent-session safety, checked and fixed.** An earlier version
  of `internal/substrate` had two real bugs here: a lock-free rewrite
  could drop a concurrent `Append` from a second Claude Code session in
  the same project, and separately, acknowledging happened before
  printing — a crash between the two could mark a finding "seen" that
  was never shown. Both closed by `internal/substrate/lock.go` (an
  `flock`-based lock, Linux-only, matching this project's existing
  scope) and by restructuring `ShowUnacknowledged` to hold that lock
  across the print itself, not just the disk rewrite — see
  `internal/substrate/substrate_test.go`'s
  `TestConcurrentAppendDuringShow` and
  `TestShowFailureDoesNotAcknowledge` for the actual reproductions, not
  just the claim that it's fixed.
