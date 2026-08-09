# capitulant

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

No production classifier exists. A live integration exists
([`hook/`](hook/)) but has no live calibration data yet — read its
known gaps before trusting it on a project you rely on. This
is a prototype and a literature review, not a tool.

- **Classifier.** A prompt against gpt-oss-20b — a prompt, not a
  trained model or a service. Mean 90.7% accuracy (range 89.7–93.1%
  across ten archived reruns —
  [`eval/results/seed29_10runs.json`](eval/results/seed29_10runs.json))
  against the eval set below.
- **Eval set.** 29 cases in [`eval/seed_set.jsonl`](eval/seed_set.jsonl),
  ground truth by construction, not by annotation. Three are real (one
  primary-source transcript, quoted and cited); the rest are synthetic
  and say so in their own metadata.
- **Real-world pass.** Run once against 1,019 blocks of actual local
  transcript text. Raw flag rate: 11/1,019. Flag rate after a human
  read the eleven: roughly 1–2. Most of the rest were the classifier
  pattern-matching "obstacle, then pivot" language that ordinary
  debugging narration produces constantly and mostly innocently — a
  raw flag count from this classifier should never be reported without
  a human reading what got flagged.

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

## Limitations

- No independent hand-labeled corpus beyond the 3 real seed cases.
- No peer-strength second judge — the only cross-family attempt so
  far (qwen3:1.7b) was too weak to trust, and that's on the record in
  `LIT_REVIEW.md` rather than quietly dropped.
- An open critique from adjacent prior art (Hodoscope, cited in
  `LIT_REVIEW.md`): a fixed-category classifier like this one will
  miss misbehavior outside its named categories, by design. A real
  tradeoff this project hasn't yet argued is the right one.
- No answer yet to where this plugs in, if it ever does: an
  agent-harness hook, a standalone service, or exactly what it is
  today — an offline transcript reader. See
  [`LIT_REVIEW.md`'s "Roadmap"](LIT_REVIEW.md#roadmap) for what's
  built vs. still open ambition.

Nothing here runs against anyone's data but the maintainer's own, and
nothing here is wired into anything that acts on its output.

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
