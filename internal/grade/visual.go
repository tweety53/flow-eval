package grade

import (
	"context"
	"regexp"
	"strings"

	"github.com/tweety53/flow-eval/internal/harness"
	"github.com/tweety53/flow-eval/internal/metrics"
)

// VisualDefect is one labelled departure of a mismatch case.
type VisualDefect struct {
	Summary string `json:"summary"`
}

// VisualLabels is a visual case's labels.json (design.md §3).
type VisualLabels struct {
	Verdict string         `json:"verdict"` // match or mismatch
	Defects []VisualDefect `json:"defects"`
}

// Visual is a visual trial's grade.
type Visual struct {
	Metrics metrics.Visual
	// JudgeUsage is the judge's total usage over this trial, reported apart from the trial's own.
	JudgeUsage harness.Usage
}

var (
	// _verdictLine is the prompt's closing `verdict: match|mismatch` line, emphasis tolerated.
	_verdictLine = regexp.MustCompile("(?im)^[\\s*_`]*verdict[*_`]*:[\\s*_`]*(match|mismatch)[\\s*_`.]*$")
	_defectLine  = regexp.MustCompile(`(?m)^\s*(?:[-*]|\d+[.)])\s+[*_]*defect[*_]*:[*_]*\s*(.+?)\s*$`)
)

// ParseVisual returns the verdict of a verifier's report — its last verdict line, "" when it has
// none — and the departures it listed as `- defect: <text>` lines.
func ParseVisual(report string) (verdict string, defects []string) {
	if all := _verdictLine.FindAllStringSubmatch(report, -1); all != nil {
		verdict = strings.ToLower(all[len(all)-1][1])
	}
	for _, m := range _defectLine.FindAllStringSubmatch(report, -1) {
		defects = append(defects, m[1])
	}
	return verdict, defects
}

// GradeVisual scores report against labels per design.md §5: correct when its verdict equals the
// labelled one, which a report with no verdict never is, false_pass on a match reported for a mismatch case,
// and for a mismatch case, localization: the share of labelled defects the judge matches to a
// reported defect.
func GradeVisual(ctx context.Context, report string, labels VisualLabels, j Judge) (Visual, error) {
	var v Visual
	verdict, reported := ParseVisual(report)
	v.Metrics.Label = labels.Verdict
	v.Metrics.Correct = verdict == labels.Verdict
	v.Metrics.FalsePass = verdict == "match" && labels.Verdict == "mismatch"
	if labels.Verdict != "mismatch" {
		return v, nil
	}
	located := 0
	for _, d := range labels.Defects {
		for _, r := range reported {
			same, u, err := j.Same(ctx, d.Summary, r)
			addUsage(&v.JudgeUsage, u)
			if err != nil {
				return v, err
			}
			if same {
				located++
				break
			}
		}
	}
	v.Metrics.Localization = ratio(located, len(labels.Defects), 0)
	return v, nil
}
