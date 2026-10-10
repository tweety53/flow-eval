// Package compare decides whether a candidate run is non-inferior to its role's baseline run
// (design.md §7), and writes the comparison report.
package compare

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tweety53/flow-eval/internal/grade"
	"github.com/tweety53/flow-eval/internal/metrics"
	"github.com/tweety53/flow-eval/internal/run"
)

// Exit codes of a comparison (design.md §7); a run pair that cannot be compared is exit 2.
const (
	ExitNonInferior = 0
	ExitRegression  = 1
	ExitImprovement = 3
)

// Run is a recorded run: runs/<ID>.jsonl.
type Run struct {
	ID     string
	Header run.Header
	Lines  []run.Line
}

// Load reads runs/<id>.jsonl under runsDir.
func Load(runsDir, id string) (Run, error) {
	h, lines, err := run.Load(runsDir, id)
	if err != nil {
		return Run{}, err
	}
	return Run{ID: id, Header: h, Lines: lines}, nil
}

// caseStats is one case's valid trials summarized.
type caseStats struct {
	metrics.Aggregate
	falsePasses int
}

// side is one run reduced to what the pass rule reads.
type side struct {
	cases       map[string]caseStats
	score       float64 // metrics.RoleScore of the cases
	cost        float64
	passes      int
	falsePasses int
}

func summarize(r Run) side {
	byCase := map[string][]metrics.Trial{}
	for _, l := range r.Lines {
		byCase[l.Case] = append(byCase[l.Case], l.Trial)
	}
	s := side{cases: map[string]caseStats{}}
	var scored [][]metrics.Trial
	for _, id := range slices.Sorted(maps.Keys(r.Header.CaseHashes)) {
		cs := caseStats{Aggregate: metrics.CaseAggregate(byCase[id])}
		for _, t := range byCase[id] {
			if !t.InfraError && t.Visual != nil && t.Visual.FalsePass {
				cs.falsePasses++
			}
		}
		s.cases[id] = cs
		scored = append(scored, byCase[id])
		s.cost += cs.Cost
		s.passes += cs.Passes
		s.falsePasses += cs.falsePasses
	}
	s.score, _ = metrics.RoleScore(scored)
	return s
}

// Decision is a comparison's outcome.
type Decision struct {
	Exit    int
	Verdict string // one line: non-inferior, regression or improvement, with the reasons
	// FlowEval is one line naming both runs' flow-eval shas, flagged when they differ: a grader
	// change between the runs can move scores, so a difference is shown, never refused.
	FlowEval string
	// Judges is a warning naming both runs' judges when they differ or a review or visual run records
	// none, since a judge it does not record may differ; "" otherwise, as for impl, which calls no
	// judge. A warning never refuses the comparison nor changes its exit.
	Judges string
	// Prompts is a warning naming both runs' prompt pins when they differ, "" otherwise. Like Judges,
	// it never refuses the comparison nor changes its exit.
	Prompts string
	Report  string // the Markdown report
}

// Warnings returns the decision's non-empty warnings in the order compare prints them.
func (d Decision) Warnings() []string {
	var ws []string
	for _, w := range []string{d.Prompts, d.Judges} {
		if w != "" {
			ws = append(ws, w)
		}
	}
	return ws
}

// Compare applies the pass rule of design.md §7 to cand against base with margin in score points.
// err, exit 2, is a pair that cannot be compared: a margin that is not positive, another role or slot,
// another case set (naming the differing cases), another k, or a case with fewer than k valid trials
// on either side: run.Comparable.
func Compare(cand, base Run, margin float64) (Decision, error) {
	if margin <= 0 {
		return Decision{}, fmt.Errorf("compare %s with %s: margin %g must be positive", cand.ID, base.ID, margin)
	}
	ch, bh := cand.Header, base.Header
	if err := run.Comparable(cand.ID, ch, cand.Lines, base.ID, bh, base.Lines); err != nil {
		return Decision{}, err
	}
	cs, bs := summarize(cand), summarize(base)

	var regressions, improvements, flips []string
	if cs.score < bs.score-margin {
		regressions = append(regressions, fmt.Sprintf("role score %.1f < %.1f − %g", cs.score, bs.score, margin))
	}
	if cs.score >= bs.score+margin {
		improvements = append(improvements, fmt.Sprintf("role score %.1f ≥ %.1f + %g", cs.score, bs.score, margin))
	}
	ids := slices.Sorted(maps.Keys(bh.CaseHashes))
	for _, id := range ids {
		c, b := cs.cases[id], bs.cases[id]
		switch {
		case b.PassK && c.Passes == 0:
			f := id + ": pass^k → 0"
			regressions, flips = append(regressions, f), append(flips, f)
		case b.Passes == 0 && c.PassK:
			f := id + ": 0 → pass^k"
			improvements, flips = append(improvements, f), append(flips, f)
		}
	}
	if ch.Role == "visual" && cs.falsePasses > bs.falsePasses {
		regressions = append(regressions, fmt.Sprintf("false-pass %d > %d", cs.falsePasses, bs.falsePasses))
	}

	d := Decision{Exit: ExitNonInferior, Verdict: "non-inferior"}
	switch {
	case len(regressions) > 0:
		d.Exit, d.Verdict = ExitRegression, "regression: "+strings.Join(regressions, "; ")
	case len(improvements) > 0:
		d.Exit, d.Verdict = ExitImprovement, "improvement to review: "+strings.Join(improvements, "; ")
	}
	d.FlowEval = fmt.Sprintf("flow-eval %s vs %s", ch.FlowEvalSHA, bh.FlowEvalSHA)
	if ch.FlowEvalSHA != bh.FlowEvalSHA {
		d.FlowEval += " — DIFFER: grading may have changed between the runs"
	}
	switch {
	case ch.Judge != bh.Judge:
		d.Judges = fmt.Sprintf("warning: judge %s vs %s — DIFFER: scores may not be comparable", ch.Judge, bh.Judge)
	case ch.Role != "impl" && ch.Judge == (grade.Pin{}):
		d.Judges = fmt.Sprintf("warning: judge %s vs %s — UNRECORDED: scores may not be comparable", ch.Judge, bh.Judge)
	}
	if ch.PromptPin != bh.PromptPin {
		d.Prompts = fmt.Sprintf("warning: prompt pin %s vs %s — DIFFER: prompts may have changed between the runs",
			ch.PromptPin, bh.PromptPin)
	}
	d.Report = report(cand, base, cs, bs, ids, flips, margin, d)
	return d, nil
}

func costPerPass(s side) string {
	if s.passes == 0 {
		return "–"
	}
	return fmt.Sprintf("$%.4f", s.cost/float64(s.passes))
}

func report(cand, base Run, cs, bs side, ids, flips []string, margin float64, d Decision) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s vs %s\n\n", cand.ID, base.ID)
	fmt.Fprintf(&b, "- role `%s`, margin %g points\n", cand.Header.Role, margin)
	fmt.Fprintf(&b, "- candidate `%s` `%s` `%s`; baseline `%s` `%s` `%s`\n",
		cand.Header.Harness, cand.Header.Model, cand.Header.Effort, base.Header.Harness, base.Header.Model, base.Header.Effort)
	fmt.Fprintf(&b, "- %s\n", d.FlowEval)
	for _, w := range d.Warnings() {
		fmt.Fprintf(&b, "- %s\n", w)
	}
	fmt.Fprintf(&b, "- role score %.1f vs %.1f (%+.1f)\n", cs.score, bs.score, cs.score-bs.score)
	fmt.Fprintf(&b, "- cost per pass %s vs %s\n", costPerPass(cs), costPerPass(bs))
	if cand.Header.Role == "visual" {
		fmt.Fprintf(&b, "- false-pass %d vs %d\n", cs.falsePasses, bs.falsePasses)
	}
	b.WriteString("\n| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, id := range ids {
		c, s := cs.cases[id], bs.cases[id]
		fmt.Fprintf(&b, "| %s | %.1f | %.1f | %+.1f | $%.4f | $%.4f | %.1f | %.1f | %d | %d |\n",
			id, c.MeanScore, s.MeanScore, c.MeanScore-s.MeanScore, c.Cost, s.Cost,
			float64(c.WallMS)/float64(c.Valid)/1000, float64(s.WallMS)/float64(s.Valid)/1000, c.Tokens, s.Tokens)
	}
	b.WriteString("\n## Flips\n\n")
	if len(flips) == 0 {
		b.WriteString("None.\n")
	}
	for _, f := range flips {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	fmt.Fprintf(&b, "\n**Verdict:** %s\n", d.Verdict)
	return b.String()
}

// WriteReport writes d's report to runsDir/<cand>-vs-<base>.md and returns its path.
func WriteReport(runsDir, candID, baseID string, d Decision) (string, error) {
	if d.Report == "" {
		return "", errors.New("compare: no report to write")
	}
	path := filepath.Join(runsDir, candID+"-vs-"+baseID+".md")
	return path, os.WriteFile(path, []byte(d.Report), 0o644)
}
