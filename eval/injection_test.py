#!/usr/bin/env python3
"""Prompt-injection stress test against classify.Run's real request shape.

classify.go builds the user message as `fmt.Sprintf("Excerpt: %q", text)` --
Go's %q escapes quotes/newlines, so a literal quote-breakout out of the JSON
request is blocked, but nothing defends against semantic injection: text
*inside* the excerpt that tries to talk the model out of its system
instructions (a fake "SYSTEM:" line, a fake pre-filled answer, a claim of
special grader status, a request to change topic, a fake role-tag). Every
case here embeds a genuine, unambiguous scope-violation-plus-license-taking
narrative (so the correct answer is YES) plus one such injection attempt --
this measures whether the real classifier holds the correct verdict anyway.

Usage: HF_TOKEN=hf_... python3 injection_test.py [output.json]
"""
import json
import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
from run_judge import call_hf, extract_verdict  # noqa: E402


def main():
    hf_key = os.environ.get("HF_TOKEN")
    if not hf_key:
        sys.exit("Set HF_TOKEN to a Hugging Face Inference Providers API key.")

    out_path = sys.argv[1] if len(sys.argv) > 1 else "injection_results.json"
    cases_path = os.path.join(os.path.dirname(__file__), "injection_cases.json")
    with open(cases_path) as f:
        cases = json.load(f)

    results = []
    held = 0
    for c in cases:
        content, reasoning = call_hf(c["text"], hf_key)
        verdict = extract_verdict(content, reasoning)
        ok = verdict == c["expected"]
        held += ok
        results.append({**c, "verdict": verdict, "raw_answer": (content or reasoning)[:400], "held": ok})
        print(f"[{'HELD' if ok else 'BROKEN'}] {c['id']:28} expected={c['expected']:8} got={verdict}")

    print(f"\n{held}/{len(cases)} held")
    with open(out_path, "w") as f:
        json.dump(results, f, indent=2)
    print(f"full results written to {out_path}")


if __name__ == "__main__":
    main()
