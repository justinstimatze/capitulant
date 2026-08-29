package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/justinstimatze/capitulant/hook/internal/classify"
)

// cmdBench is the Go equivalent of eval/run_judge.py, against
// classify.Run's Anthropic backend instead of run_judge.py's HF one.
// Built as a repeatable command rather than a one-off script because
// this backend's accuracy needs re-checking after any prompt, model, or
// seed-set change -- not just once at port time.
func cmdBench(args []string) {
	fl := flag.NewFlagSet("bench", flag.ContinueOnError)
	fl.SetOutput(os.Stderr)
	seedPath := fl.String("seed", "../eval/seed_set.jsonl", "path to the seed set JSONL (default assumes running from hook/)")
	outPath := fl.String("out", "", "path to write full results JSON (required)")
	retries := fl.Int("retries", 4, "retries per case on a transient API error")
	if err := fl.Parse(args); err != nil {
		os.Exit(2)
	}
	if *outPath == "" {
		fmt.Fprintln(os.Stderr, "capitulant-hook bench: -out is required, e.g. -out ../eval/results/seed37_haiku_run1.json")
		os.Exit(2)
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "capitulant-hook bench: ANTHROPIC_API_KEY not set")
		os.Exit(1)
	}

	cases, err := readSeedSet(*seedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "capitulant-hook bench: reading seed set: %s\n", err)
		os.Exit(1)
	}

	results := make([]benchResult, 0, len(cases))
	for _, c := range cases {
		expected, ok := labelToExpected[c.Label]
		if !ok {
			fmt.Fprintf(os.Stderr, "capitulant-hook bench: %s: unknown label %q\n", c.ID, c.Label)
			os.Exit(1)
		}

		res, err := runWithRetries(c.Text, apiKey, *retries)
		if err != nil {
			fmt.Fprintf(os.Stderr, "capitulant-hook bench: %s: giving up: %s\n", c.ID, err)
			os.Exit(1)
		}

		br := benchResult{
			seedCase:     c,
			Judge:        benchModel,
			ModelAnswer:  res.Notes,
			ModelVerdict: res.Verdict,
			Expected:     expected,
			Correct:      res.Verdict == expected,
		}
		if expected == "YES" {
			br.ModelCategory = res.Category
			catOK := res.Category == c.Category
			br.CategoryCorrect = &catOK
		}
		results = append(results, br)

		mark := "MISS"
		if br.Correct {
			mark = "OK "
		}
		fmt.Printf("[%s] %-14s exp=%-8s got=%-8s\n", mark, c.ID, expected, res.Verdict)
	}

	out, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "capitulant-hook bench: marshaling results: %s\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outPath, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "capitulant-hook bench: writing %s: %s\n", *outPath, err)
		os.Exit(1)
	}

	n, nc, ncat, ncatTotal := 0, 0, 0, 0
	for _, r := range results {
		n++
		if r.Correct {
			nc++
		}
		if r.CategoryCorrect != nil {
			ncatTotal++
			if *r.CategoryCorrect {
				ncat++
			}
		}
	}
	fmt.Printf("\n%s: %d/%d = %.1f%%\n", benchModel, nc, n, 100*float64(nc)/float64(n))
	if ncatTotal > 0 {
		fmt.Printf("category (on YES cases only): %d/%d = %.1f%%\n", ncat, ncatTotal, 100*float64(ncat)/float64(ncatTotal))
	}
	fmt.Printf("\nfull results written to %s\n", *outPath)
}

// benchModel names the classifier under test in bench's output. Kept
// separate from classify.model (unexported) since a results file should
// name what was actually scored even if classify.go's model constant
// changes later.
const benchModel = "claude-haiku-4-5-20251001"

var labelToExpected = map[string]string{
	"positive":                 "YES",
	"negative":                 "NO",
	"not_detectable_from_text": "UNCLEAR",
}

type seedCase struct {
	ID         string `json:"id"`
	Category   string `json:"category"`
	Label      string `json:"label"`
	Provenance string `json:"provenance"`
	Source     string `json:"source"`
	Text       string `json:"text"`
	Rationale  string `json:"rationale"`
}

type benchResult struct {
	seedCase
	Judge           string `json:"judge"`
	ModelAnswer     string `json:"model_answer"`
	ModelVerdict    string `json:"model_verdict"`
	Expected        string `json:"expected"`
	Correct         bool   `json:"correct"`
	ModelCategory   string `json:"model_category,omitempty"`
	CategoryCorrect *bool  `json:"category_correct,omitempty"`
}

// runWithRetries mirrors run_judge.py's call_hf retry behavior: back off
// and retry on a transient error, but fail fast on an auth error rather
// than burning the full retry budget against a key that will never work.
func runWithRetries(text, apiKey string, retries int) (classify.Result, error) {
	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		res, err := classify.Run(text, apiKey)
		if err == nil {
			return res, nil
		}
		if strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "HTTP 403") {
			return classify.Result{}, fmt.Errorf("auth rejected, not retrying: %w", err)
		}
		lastErr = err
		fmt.Fprintf(os.Stderr, "  retry %d/%d after error: %s\n", attempt+1, retries, err)
		time.Sleep(time.Duration(2*(attempt+1)) * time.Second)
	}
	return classify.Result{}, lastErr
}

func readSeedSet(path string) ([]seedCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []seedCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var c seedCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("parsing line: %w", err)
		}
		cases = append(cases, c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return cases, nil
}
