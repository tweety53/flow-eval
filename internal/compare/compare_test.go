package compare

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/tweety53/flow-eval/internal/grade"
	"github.com/tweety53/flow-eval/internal/metrics"
	"github.com/tweety53/flow-eval/internal/run"
)

// fixture builds a run of role whose case id has one trial per score; a trial passes at 100. Every
// case's hash is "h-<id>" and the prompt pin is "pin".
func fixture(id, role string, k int, scores map[string][]float64) Run {
	r := Run{ID: id, Header: run.Header{
		Role: role, K: k, PromptPin: "pin", CaseHashes: map[string]string{},
	}}
	for _, c := range slices.Sorted(maps.Keys(scores)) {
		r.Header.CaseHashes[c] = "h-" + c
		for i, s := range scores[c] {
			t := metrics.Trial{Score: s, Passed: s == 100, CostUSD: 1, WallMS: 1000, Input: 10}
			if role == "visual" {
				t.Visual = &metrics.Visual{Correct: s == 100}
			}
			r.Lines = append(r.Lines, run.Line{Case: c, TrialNum: i + 1, Trial: t})
		}
	}
	r.Header.CaseSetHash = strings.Join(slices.Sorted(maps.Values(r.Header.CaseHashes)), ",")
	return r
}

func decide(t *testing.T, cand, base Run) Decision {
	t.Helper()
	d, err := Compare(cand, base, 5)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	return d
}

func TestNonInferiorWithinMargin(t *testing.T) {
	base := fixture("base", "review", 3, map[string][]float64{"a": {80, 80, 80}, "b": {100, 100, 100}})
	cand := fixture("cand", "review", 3, map[string][]float64{"a": {72, 72, 72}, "b": {100, 100, 100}})
	// Role score 90 → 86: a 4-point drop, within the 5-point margin.
	d := decide(t, cand, base)
	if d.Exit != 0 || !strings.Contains(d.Verdict, "non-inferior") {
		t.Errorf("exit %d (%s), want 0", d.Exit, d.Verdict)
	}
	// Three trials of $1, 1s and 10 tokens each, on both sides.
	if row := "| a | 72.0 | 80.0 | -8.0 | $3.0000 | $3.0000 | 1.0 | 1.0 | 30 | 30 |"; !strings.Contains(d.Report, row) {
		t.Errorf("report lacks the row %q:\n%s", row, d.Report)
	}
	// $6 over b's three passes on each side.
	if line := "- cost per pass $2.0000 vs $2.0000"; !strings.Contains(d.Report, line) {
		t.Errorf("report lacks %q:\n%s", line, d.Report)
	}
}

func TestRegressionBeyondMargin(t *testing.T) {
	base := fixture("base", "review", 3, map[string][]float64{"a": {80, 80, 80}, "b": {60, 60, 60}})
	cand := fixture("cand", "review", 3, map[string][]float64{"a": {69, 69, 69}, "b": {60, 60, 60}})
	// Role score 70 → 64.5: a 5.5-point drop; a drop of exactly the margin still passes.
	if d := decide(t, cand, base); d.Exit != 1 || !strings.Contains(d.Verdict, "role score") {
		t.Errorf("exit %d (%s), want 1 for a 5.5-point drop", d.Exit, d.Verdict)
	}
	cand = fixture("cand", "review", 3, map[string][]float64{"a": {70, 70, 70}, "b": {60, 60, 60}})
	if d := decide(t, cand, base); d.Exit != 0 {
		t.Errorf("exit %d (%s), want 0 for a drop of exactly the margin", d.Exit, d.Verdict)
	}
}

func TestAlwaysPassFlipIsRegression(t *testing.T) {
	scores := map[string][]float64{"a": {100, 100, 100}}
	for _, c := range []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9", "c10", "c11", "c12", "c13", "c14", "c15", "c16", "c17", "c18", "c19", "c20", "c21"} {
		scores[c] = []float64{100, 100, 100}
	}
	base := fixture("base", "impl", 3, scores)
	scores["a"] = []float64{0, 0, 0} // role score drops by 100/22 = 4.5 points: within the margin
	cand := fixture("cand", "impl", 3, scores)
	if d := decide(t, cand, base); d.Exit != 1 || !strings.Contains(d.Verdict, "a: pass^k → 0") {
		t.Errorf("exit %d (%s), want 1 naming the flip of a", d.Exit, d.Verdict)
	}
}

func TestVisualFalsePassIncreaseIsRegression(t *testing.T) {
	base := fixture("base", "visual", 2, map[string][]float64{"m": {100, 0}, "n": {100, 100}})
	cand := fixture("cand", "visual", 2, map[string][]float64{"m": {100, 0}, "n": {100, 100}})
	// The same score, but the candidate's miss is a false pass and the baseline's is not.
	cand.Lines[1].Visual.FalsePass = true
	if d := decide(t, cand, base); d.Exit != 1 || !strings.Contains(d.Verdict, "false-pass") {
		t.Errorf("exit %d (%s), want 1 for a false-pass increase", d.Exit, d.Verdict)
	}
}

func TestInfraErrorFalsePassIsNotCounted(t *testing.T) {
	base := fixture("base", "visual", 2, map[string][]float64{"m": {100, 0}})
	cand := fixture("cand", "visual", 2, map[string][]float64{"m": {100, 0}})
	cand.Lines = append(cand.Lines, run.Line{Case: "m", TrialNum: 3, Trial: metrics.Trial{
		InfraError: true, Visual: &metrics.Visual{FalsePass: true},
	}})
	if d := decide(t, cand, base); d.Exit != 0 {
		t.Errorf("exit %d (%s), want 0: an infra-error trial's false pass is not scored", d.Exit, d.Verdict)
	}
}

// The visual role score is balanced accuracy: a verifier that always answers mismatch, right on 12
// mismatch cases and wrong on the one match case, scores 50, not 92, against a baseline of 90.
func TestVisualRoleScoreIsBalancedAccuracy(t *testing.T) {
	base, cand := map[string][]float64{"m": {90}}, map[string][]float64{"m": {0}}
	for i := range 12 {
		base[fmt.Sprint("x", i)], cand[fmt.Sprint("x", i)] = []float64{90}, []float64{100}
	}
	label := func(r Run) Run {
		for _, l := range r.Lines {
			l.Visual.Label = "mismatch"
			if l.Case == "m" {
				l.Visual.Label = "match"
			}
		}
		return r
	}
	d := decide(t, label(fixture("cand", "visual", 1, cand)), label(fixture("base", "visual", 1, base)))
	if d.Exit != 1 || !strings.Contains(d.Verdict, "role score 50.0 < 90.0") {
		t.Errorf("exit %d (%s), want 1 for a balanced score of 50 against 90", d.Exit, d.Verdict)
	}
}

func TestImprovementExit3(t *testing.T) {
	base := fixture("base", "review", 3, map[string][]float64{"a": {50, 50, 50}})
	cand := fixture("cand", "review", 3, map[string][]float64{"a": {60, 60, 60}})
	if d := decide(t, cand, base); d.Exit != 3 {
		t.Errorf("exit %d (%s), want 3 for a 10-point gain", d.Exit, d.Verdict)
	}
	base = fixture("base", "impl", 3, map[string][]float64{"a": {0, 0, 0}, "b": {100, 100, 0}})
	cand = fixture("cand", "impl", 3, map[string][]float64{"a": {100, 100, 100}, "b": {100, 0, 0}})
	if d := decide(t, cand, base); d.Exit != 3 || !strings.Contains(d.Verdict, "a: 0 → pass^k") {
		t.Errorf("exit %d (%s), want 3 for a 0 → pass^k flip", d.Exit, d.Verdict)
	}
}

func TestRegressionBeatsImprovement(t *testing.T) {
	scores := map[string][]float64{"up": {0, 0, 0}, "down": {100, 100, 100}}
	base := fixture("base", "impl", 3, scores)
	cand := fixture("cand", "impl", 3, map[string][]float64{"up": {100, 100, 100}, "down": {0, 0, 0}})
	if d := decide(t, cand, base); d.Exit != 1 {
		t.Errorf("exit %d (%s), want 1: a regression wins over an improvement", d.Exit, d.Verdict)
	}
}

func TestCaseSetMismatchExit2(t *testing.T) {
	base := fixture("base", "impl", 1, map[string][]float64{"a": {100}})
	cand := fixture("cand", "impl", 1, map[string][]float64{"a": {100}, "extra": {100}})
	cand.Header.CaseHashes["a"] = "changed"
	cand.Header.CaseSetHash = "different"
	_, err := Compare(cand, base, 5)
	if err == nil || !strings.Contains(err.Error(), "a, extra") {
		t.Errorf("err %v, want a case-set refusal naming a and extra", err)
	}
}

func TestMissingTrialsExit2(t *testing.T) {
	base := fixture("base", "impl", 3, map[string][]float64{"a": {100, 100, 100}})
	cand := fixture("cand", "impl", 3, map[string][]float64{"a": {100, 100, 100}})
	cand.Lines[2].InfraError = true
	if _, err := Compare(cand, base, 5); err == nil || !strings.Contains(err.Error(), "a") {
		t.Errorf("err %v, want a refusal naming case a's missing trial", err)
	}
	cand = fixture("cand", "impl", 3, map[string][]float64{"a": {100, 100, 100}})
	base.Lines = base.Lines[:1]
	if _, err := Compare(cand, base, 5); err == nil || !strings.Contains(err.Error(), "base") {
		t.Errorf("err %v, want a refusal naming the baseline run", err)
	}
}

func TestKOrMarginMismatchExit2(t *testing.T) {
	base := fixture("base", "impl", 3, map[string][]float64{"a": {0, 0, 0}})
	cand := fixture("cand", "impl", 1, map[string][]float64{"a": {100}})
	if _, err := Compare(cand, base, 5); err == nil || !strings.Contains(err.Error(), "k 1 against 3") {
		t.Errorf("err %v, want a refusal naming k 1 against 3", err)
	}
	for _, margin := range []float64{0, -5} {
		if _, err := Compare(base, base, margin); err == nil || !strings.Contains(err.Error(), "must be positive") {
			t.Errorf("margin %g: err %v, want a refusal", margin, err)
		}
	}
}

func TestFlowEvalShasArePrintedAndADifferenceFlagged(t *testing.T) {
	scores := map[string][]float64{"a": {100, 100, 100}}
	for _, tc := range []struct {
		name, candSHA, want string
	}{
		{"same sha", "s1", "flow-eval s1 vs s1"},
		{"different sha", "s2", "flow-eval s2 vs s1 — DIFFER: grading may have changed between the runs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, cand := fixture("base", "review", 3, scores), fixture("cand", "review", 3, scores)
			base.Header.FlowEvalSHA, cand.Header.FlowEvalSHA = "s1", tc.candSHA
			d := decide(t, cand, base)
			if d.Exit != 0 || d.FlowEval != tc.want || !strings.Contains(d.Report, "- "+tc.want+"\n") {
				t.Errorf("exit %d, FlowEval %q, report:\n%s\nwant exit 0 and %q in both", d.Exit, d.FlowEval, d.Report, tc.want)
			}
		})
	}
}

func TestCompareWarnsOnMixedJudges(t *testing.T) {
	scores := map[string][]float64{"a": {100, 100, 100}}
	sonnet := grade.Pin{Harness: "claude-code", Model: "sonnet", Effort: "medium"}
	opus := grade.Pin{Harness: "claude-code", Model: "opus", Effort: "medium"}
	for _, tc := range []struct {
		name, role string
		cand, base grade.Pin
		want       string
	}{
		{"same judge", "review", sonnet, sonnet, ""},
		{"different judges", "review", sonnet, opus,
			"warning: judge claude-code sonnet medium vs claude-code opus medium — DIFFER: scores may not be comparable"},
		{"baseline records none", "review", sonnet, grade.Pin{},
			"warning: judge claude-code sonnet medium vs – — DIFFER: scores may not be comparable"},
		{"both record none", "visual", grade.Pin{}, grade.Pin{},
			"warning: judge – vs – — UNRECORDED: scores may not be comparable"},
		{"impl, which calls no judge", "impl", grade.Pin{}, grade.Pin{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, cand := fixture("base", tc.role, 3, scores), fixture("cand", tc.role, 3, scores)
			base.Header.Judge, cand.Header.Judge = tc.base, tc.cand
			d := decide(t, cand, base)
			if d.Exit != ExitNonInferior || d.Verdict != "non-inferior" || d.Judges != tc.want {
				t.Errorf("exit %d (%s), Judges %q; want exit 0, non-inferior, %q", d.Exit, d.Verdict, d.Judges, tc.want)
			}
			if inReport := strings.Contains(d.Report, "- warning: judge"); inReport != (tc.want != "") ||
				tc.want != "" && !strings.Contains(d.Report, "- "+tc.want+"\n") {
				t.Errorf("report:\n%s\nwant the warning %q exactly when the judges differ", d.Report, tc.want)
			}
		})
	}
}

func TestCompareWarnsOnMixedPromptPins(t *testing.T) {
	const warning = "warning: prompt pin p2 vs p1 — DIFFER: prompts may have changed between the runs"
	for _, tc := range []struct {
		name, candPin string
		candScores    []float64
		exit          int
		want          string
	}{
		{"same pin", "p1", []float64{100, 100, 100}, ExitNonInferior, ""},
		{"different pins", "p2", []float64{100, 100, 100}, ExitNonInferior, warning},
		{"different pins, a regression", "p2", []float64{0, 0, 0}, ExitRegression, warning},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := fixture("base", "review", 3, map[string][]float64{"a": {100, 100, 100}})
			cand := fixture("cand", "review", 3, map[string][]float64{"a": tc.candScores})
			base.Header.PromptPin, cand.Header.PromptPin = "p1", tc.candPin
			d := decide(t, cand, base)
			if d.Exit != tc.exit || d.Prompts != tc.want {
				t.Errorf("exit %d (%s), Prompts %q; want exit %d and %q", d.Exit, d.Verdict, d.Prompts, tc.exit, tc.want)
			}
			if inReport := strings.Contains(d.Report, "- warning: prompt pin"); inReport != (tc.want != "") ||
				tc.want != "" && !strings.Contains(d.Report, "- "+tc.want+"\n") {
				t.Errorf("report:\n%s\nwant the warning %q exactly when the pins differ", d.Report, tc.want)
			}
		})
	}
}

func TestCompareWarnsOnDifferentServedModels(t *testing.T) {
	const warning = "warning: served model m2 vs m1 — DIFFER: the runs were served different models"
	for _, tc := range []struct {
		name, candServed string
		candScores       []float64
		exit             int
		want             string
	}{
		{"same served model", "m1", []float64{100, 100, 100}, ExitNonInferior, ""},
		{"different served models", "m2", []float64{100, 100, 100}, ExitNonInferior, warning},
		{"different served models, a regression", "m2", []float64{0, 0, 0}, ExitRegression, warning},
		{"candidate names none", "", []float64{100, 100, 100}, ExitNonInferior, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := fixture("base", "review", 3, map[string][]float64{"a": {100, 100, 100}})
			cand := fixture("cand", "review", 3, map[string][]float64{"a": tc.candScores})
			for i := range base.Lines {
				base.Lines[i].ServedModel, cand.Lines[i].ServedModel = "m1", tc.candServed
			}
			d := decide(t, cand, base)
			if d.Exit != tc.exit || d.Served != tc.want {
				t.Errorf("exit %d (%s), Served %q; want exit %d and %q", d.Exit, d.Verdict, d.Served, tc.exit, tc.want)
			}
			if inReport := strings.Contains(d.Report, "- warning: served model"); inReport != (tc.want != "") ||
				tc.want != "" && !strings.Contains(d.Report, "- "+tc.want+"\n") {
				t.Errorf("report:\n%s\nwant the warning %q exactly when the served models differ", d.Report, tc.want)
			}
			line := fmt.Sprintf("served %s; baseline `` `` `` served m1\n", cmp.Or(tc.candServed, "–"))
			if !strings.Contains(d.Report, line) {
				t.Errorf("report lacks %q:\n%s", line, d.Report)
			}
		})
	}
}
