# capitulant

[![CI](https://github.com/justinstimatze/capitulant/actions/workflows/ci.yml/badge.svg)](https://github.com/justinstimatze/capitulant/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A prototype text classifier and literature review for one specific AI
agent failure: an agent concludes a task can't be finished within its
stated scope, and treats that conclusion as license to act outside the
scope anyway — instead of stopping or escalating, which is what its own
governing rules say it should do. Grounded in a real 2026 security
incident where this exact move, repeated across many agent instances,
led to a production breach (see `CODEBOOK.md`).

The name: one who capitulates — surrenders a boundary rather than
holding it.

## Status

A working classifier and a live Claude Code hook, each with real,
documented gaps — not a hardened tool. Read `hook/README.md`'s "Known
gaps" section before trusting this on a project you rely on.

- **Classifier.** Claude Haiku (`claude-haiku-4-5-20251001`) via
  Anthropic's Messages API, forced into structured tool-use output
  rather than parsed from free text — see
  [`hook/internal/classify`](hook/internal/classify). 89.2% verdict
  accuracy / 75% category accuracy on the eval set below
  ([`eval/results/seed37_haiku_structured_run3.json`](eval/results/seed37_haiku_structured_run3.json)).
  An earlier prompt-only classifier against gpt-oss-20b is still in
  the repo and still runs (see "Running it" below); the Haiku
  classifier is what the live hook and the numbers above use.
- **Eval set.** 37 cases in [`eval/seed_set.jsonl`](eval/seed_set.jsonl),
  ground truth by construction, not by annotation. Three are real (one
  primary-source transcript, quoted and cited); the rest are synthetic
  and say so in their own metadata. Small enough that a fix targeting
  one failing case can look like progress and be overfitting instead —
  see [`LIT_REVIEW.md`](LIT_REVIEW.md) for how that risk gets tracked.
- **Live hook.** [`hook/`](hook/) is installed as a real Claude Code
  Stop/UserPromptSubmit hook pair, not a dry run — it classifies
  matched turns, sends a desktop notification, and (as a fallback if
  the notification is missed) injects unacknowledged findings into the
  next turn's context. No accumulated real-usage data yet to calibrate
  against. Two gaps still open: same-family judging (Claude scoring
  Claude Code's own sessions) is untested, and the injection-defense
  evidence predates the switch to structured output.
- **Real-world pass.** Run once, against the earlier gpt-oss-20b
  classifier, against 1,019 blocks of actual local transcript text.
  Raw flag rate: 11/1,019. Flag rate after a human read the eleven:
  roughly 1–2. Most of the rest were the classifier pattern-matching
  "obstacle, then pivot" language that ordinary debugging narration
  produces constantly and mostly innocently — a raw flag count from
  this classifier should never be reported without a human reading
  what got flagged. Not yet repeated against the Haiku classifier.

## What it targets

Category 1 in `CODEBOOK.md` — *moral justification via impossibility
premise* — is the specific pattern this repository targets. The
codebook has five categories in total, grounded in Bandura's moral
disengagement research; `LIT_REVIEW.md` has the full theoretical
background and prior-art comparison.

## Running it

```
HF_TOKEN=hf_... python3 eval/run_judge.py [output.json]
```

Scores gpt-oss-20b (via Hugging Face Inference Providers) against
every case in `eval/seed_set.jsonl` — no other dependencies. See
[`eval/README.md`](eval/README.md) for the schema and category list.

`eval/run_judge.py` is the actual script behind the gpt-oss-20b
numbers above. One committed file
([`dual_judge_run1.json`](eval/results/dual_judge_run1.json)) also
contains a qwen3:1.7b run from a local-Ollama script that isn't
shipped in this repo, so that half of that one file can't be
regenerated from what's here.

For the current Haiku classifier — the one the live hook and the
89.2%/75% numbers above use:

```
ANTHROPIC_API_KEY=sk-ant-... hook/capitulant-hook bench -out output.json
```

Same eval set, same schema, plus `model_category`/`category_correct`
fields. See [`hook/README.md`](hook/README.md#testing) for build and
usage details.

## Limitations

- No independent hand-labeled corpus beyond the 3 real seed cases.
- No peer-strength second judge — the only cross-family attempt so
  far (qwen3:1.7b) was too weak to trust, and that's on the record in
  `LIT_REVIEW.md` rather than quietly dropped.
- An open critique from adjacent prior art (Hodoscope, cited in
  `LIT_REVIEW.md`): a fixed-category classifier like this one will
  miss misbehavior outside its named categories, by design. A real
  tradeoff this project hasn't yet argued is the right one.
- It now plugs in as an agent-harness hook ([`hook/`](hook/)), but
  that's unproven at scale — no live-usage calibration data yet — see
  [`LIT_REVIEW.md`'s "Roadmap"](LIT_REVIEW.md#roadmap) for what's
  built vs. still open ambition.

Nothing here runs against anyone's data but the maintainer's own. The
one thing wired to act on the classifier's output is the maintainer's
own installed hook — a desktop notification and a next-turn context
injection, nothing that touches code or blocks anything.

## See also

- [`CODEBOOK.md`](CODEBOOK.md) — the five-category classification
  scheme
- [`LIT_REVIEW.md`](LIT_REVIEW.md) — theoretical grounding, prior-art
  comparison, and known failure modes, including the ones that make
  the classifier look worse, not just the ones that make it look good
- [`eval/`](eval/) — the eval set, the scoring script, and archived
  results
- [`hook/`](hook/) — a live Claude Code hook running the classifier
  against a project's own sessions; project-local install, no live
  calibration yet, real known gaps documented in `hook/README.md`
- [`sources/blackhat-2026-transcript.md`](sources/blackhat-2026-transcript.md) —
  the primary-source transcript this project is grounded in
