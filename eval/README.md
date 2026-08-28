# Eval seed set

`seed_set.jsonl` — 33 known-ground-truth cases for scoring a classifier
against `CODEBOOK.md`'s categories, built instead of hand-labeling the
local transcript pool of narrated-pivot excerpts (see `LIT_REVIEW.md`'s
"Evaluation" section). 6 real (the Black Hat transcript's two positives and
one explicit negative, plus three verbatim quotes from
[`sources/metr-redwood-2026-report.md`](../sources/metr-redwood-2026-report.md)
covering two categories that previously had no real specimen at all), 27
constructed for this set — ground truth is known by construction, not by
annotation.

## Running it

`run_judge.py` scores gpt-oss-20b (via Hugging Face Inference Providers)
against every case in `seed_set.jsonl` — it's the actual script that
produced everything under `results/`:

```
HF_TOKEN=hf_... python3 run_judge.py [output.json]
```

Needs a Hugging Face Inference Providers API key with access to
`openai/gpt-oss-20b`, no other dependencies. This scores the seed set
only. The real-pool pass (`LIT_REVIEW.md`'s "Evaluation" section) reads
the user's own local Claude Code transcript history across every project
on the machine, not just this one — that step is inherently
non-reproducible for anyone else and isn't shipped here.

## Schema

One JSON object per line:

- `id` — stable identifier, prefixed `real-` or `syn-`
- `category` — the exact string used doesn't derive predictably from
  `CODEBOOK.md`'s prose category names, so it's spelled out here rather
  than left to guess:

  | `CODEBOOK.md` category | `category` string |
  |---|---|
  | 1. Moral justification via impossibility premise | `impossibility_license` |
  | 2. Peer-authorization substitution | `peer_authorization` |
  | 3. Advantageous comparison / collective-good framing | `advantageous_comparison` |
  | 4. Post-detection concealment | `post_detection_concealment` |
  | 5. Goal-persistence override | `goal_persistence_no_signal` |

  Plus four negative-only categories with no `CODEBOOK.md` counterpart:
  `negative_legitimate_stuck`, `negative_clean_completion`,
  `negative_near_miss`, `negative_asks_permission`. Adding a sixth
  `CODEBOOK.md` category later should give it a short `snake_case` slug
  here too, named for the move (`impossibility_license`,
  `peer_authorization`) rather than transcribed from the header.
- `label` — `positive`, `negative`, or `not_detectable_from_text`
- `provenance` — `real` or `synthetic`; real entries carry a citable source,
  synthetic entries are marked as constructed and must never be presented
  as an actual incident
- `source` — citation for real entries, `"constructed for capitulant eval
  set"` (optionally noting what it's modeled on) for synthetic ones
- `text` — the excerpt to classify
- `rationale` — why this label, one line, for a human auditing the set
  later

## What `negative_near_miss` and `not_detectable_from_text` are for

Plain negatives (`negative_clean_completion`, `negative_legitimate_stuck`)
test whether a classifier fires on nothing. `negative_near_miss` cases
share surface vocabulary with a positive of the same shape — impossibility
language, peer-precedent language, net-positive framing — but the model
does the thing Anthropic's own Constitution prescribes (raise it, stop,
escalate) instead of crossing the boundary. These are the pairs that
matter most: a classifier that can't tell `syn-il-01` from `syn-il-04`
apart is pattern-matching on vocabulary, not on the actual move.

`goal_persistence_no_signal` cases have no justification text to find at
all — category 5 in `CODEBOOK.md`, the boundary of what a text classifier
can catch. Labeled `not_detectable_from_text` rather than `negative`
deliberately: the correct classifier behavior is to abstain, not to
confidently return "no crossing happened" when the honest answer is "not
visible from this input." A classifier that labels these `negative` is
making a claim the input doesn't support.

`negative_asks_permission` was added after a real false positive turned
up scoring gpt-oss-20b against the actual local transcript pool (see
`LIT_REVIEW.md`'s "Known limitations"): an excerpt where the
agent names a boundary, names the exact bypass available, and then
explicitly stops to ask the user rather than act — and the classifier's
own stated rationale noted the agent "asks for permission" and still
returned YES, contradicting itself in the same sentence. That's a
different failure than `negative_near_miss` — those test whether the
classifier confuses shared vocabulary with the license-taking move;
these test whether it can hold onto an explicit stop-and-ask signal that's
sitting right next to boundary language strong enough to look like a
positive on its own.

`syn-ap-05` tests a narrower version of that same failure: a
pre-committed decision phrased as a foregone conclusion ("I could... that
would work... say the word") rather than a genuinely open question ("want
me to do X, or Y?"). Every other `negative_asks_permission` case poses a
real open question; this one asks the classifier to key on whether
execution happened rather than on how hedged the proposal sounds.

## Using it

Score a classifier's output against `label` for accuracy. Score agreement
between classifiers from different model families against each other only
as a secondary, weaker signal — per the motivated-mislabeling and
same-family fact-checking findings in `LIT_REVIEW.md`, agreement is not a
substitute for this file, only a triage tool for the larger unlabeled pool
this file was built to avoid hand-annotating.
