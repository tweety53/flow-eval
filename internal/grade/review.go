package grade

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/tweety53/flow-eval/internal/harness"
	"github.com/tweety53/flow-eval/internal/metrics"
)

// _window is how far, in lines, a finding may sit from a label in the same file and still be
// put to the judge as a candidate match (decision review-match-location-plus-judge).
const _window = 15

// Finding is one issue of a review report.
type Finding struct {
	Severity string // Critical, Important or Minor
	File     string // the issue's first path:line; empty when it names none
	Line     int
	Text     string // the issue as written
}

// Label is one labelled finding of a review case's labels.json.
type Label struct {
	ID string `json:"id"`
	// Slot is the panel slot that raised the finding; a --slot X trial is graded on slot X's
	// must-finds only.
	Slot string `json:"slot"`
	File string `json:"file"`
	Line int    `json:"line"`
	// Also lists further lines of File where the same defect sits — a stored value written from
	// several places, say — so a finding at any of them is a candidate match.
	Also    []int  `json:"also,omitempty"`
	Summary string `json:"summary"`
	// Severity is the panel's severity, Critical or Important; severity_agreement counts only labels
	// that carry one.
	Severity string `json:"severity,omitempty"`
}

// ReviewLabels is a review case's labels.json (design.md §3).
type ReviewLabels struct {
	MustFind   []Label `json:"mustFind"`
	KnownFalse []Label `json:"knownFalse"`
}

// Judge decides what a location cannot: whether two descriptions name the same defect, and
// whether an unlabelled finding is a plausible defect in the diff. Each answer comes with the
// judge's own usage, which is never the candidate's.
type Judge interface {
	Same(ctx context.Context, labelled, reported string) (bool, harness.Usage, error)
	Plausible(ctx context.Context, diff, finding string) (bool, harness.Usage, error)
}

// Review is a review trial's grade.
type Review struct {
	Metrics metrics.Review
	// JudgeUsage is the judge's total usage over this trial, reported apart from the trial's own.
	JudgeUsage harness.Usage
}

var (
	_severityHeading = regexp.MustCompile(`^####\s+(Critical|Important|Minor)\b`)
	_heading         = regexp.MustCompile(`^#{1,4}\s`)
	_subHeading      = regexp.MustCompile(`^#{5,6}\s`)
	_boldLed         = regexp.MustCompile(`^\*\*`)
	_numberedBold    = regexp.MustCompile(`^\*\*\d+[.)]\s`)
	_boldOnly        = regexp.MustCompile(`^\*\*[^*]+\*\*\s*$`)
	_listItem        = regexp.MustCompile(`^(?:\d+[.)]|[-*+])\s+`)
	_none            = regexp.MustCompile(`(?i)^[*_\s]*(none|n/a)\b`)
	// _location is a path:line whose path has a directory or an extension; a range keeps its start.
	_location = regexp.MustCompile(`([\w.-]*(?:/[\w.-]+)+|[\w-]+\.\w+):(\d+)`)
	// _hostPort is a location that is a host and a port. ponytail: a fixed list of common top-level
	// domains; a file named like one (notes.io) is misread as a host.
	_hostPort = regexp.MustCompile(`\.(com|org|net|io|dev|app|ai)$`)
)

// ParseReview returns the issues under a report's #### Critical, #### Important and #### Minor
// headings, in order. An issue opens at a ##### sub-heading, at a bold-led paragraph that names a
// path:line or opens with an item number (**3. …), or at an unindented list item, and runs over the
// lines that follow it. An issue opened by a sub-heading or a bold-led paragraph also runs over the
// list items that follow it: only the next sub-heading, bold-led issue or heading ends it, or a
// paragraph that is a bold label alone. A bold label alone ends any issue, however it opened, and
// its own items are no issue (decision parse-bold-led-bullets). An item saying only "None" is no issue. Any other heading of level four
// or above ends the severity section.
func ParseReview(report string) []Finding {
	var findings []Finding
	var severity string
	var cur *Finding
	// keepsItems: a list item continues cur, or opens nothing — cur opened at a sub-heading or a
	// bold-led paragraph, or a bold label ended it.
	keepsItems := false
	prevBlank := true
	for line := range strings.SplitSeq(report, "\n") {
		blank := strings.TrimSpace(line) == ""
		bold := prevBlank && _boldLed.MatchString(line)
		heads := _subHeading.MatchString(line) || (bold && (_location.MatchString(line) || _numberedBold.MatchString(line)))
		opens := heads || (_listItem.MatchString(line) && !keepsItems)
		prevBlank = blank || _heading.MatchString(line) // a heading ends a paragraph as a blank line does
		if bold && !heads && _boldOnly.MatchString(line) {
			// A bold-only label ("**Considered and not raised:**") ends the issue, whichever way it
			// opened; its items are none, so they open nothing until the next bold-led issue,
			// sub-heading or heading.
			cur, keepsItems = nil, true
			continue
		}
		if _heading.MatchString(line) {
			severity, cur, keepsItems = "", nil, false
			if m := _severityHeading.FindStringSubmatch(line); m != nil {
				severity = m[1]
			}
			continue
		}
		if severity == "" {
			continue
		}
		if opens {
			findings = append(findings, Finding{Severity: severity, Text: line})
			cur = &findings[len(findings)-1]
			keepsItems = heads
		} else if cur != nil {
			cur.Text += "\n" + line
		}
	}
	kept := findings[:0]
	for _, f := range findings {
		f.Text = strings.TrimSpace(f.Text)
		if _none.MatchString(_listItem.ReplaceAllString(f.Text, "")) {
			continue
		}
		for _, m := range _location.FindAllStringSubmatch(f.Text, -1) {
			if _hostPort.MatchString(m[1]) {
				continue
			}
			f.File = cleanPath(m[1])
			f.Line, _ = strconv.Atoi(m[2]) // the pattern admits digits only
			break
		}
		kept = append(kept, f)
	}
	return kept
}

func cleanPath(p string) string {
	return strings.TrimPrefix(path.Clean(p), "./")
}

// near reports whether f sits in l's file within _window lines of its line or of one of its also
// lines. A reviewer may name the file from the trial's working tree root (/work/a/b.go), relative to
// it (a/b.go) or by a shorter tail (b.go): the file matches when one path is the other's tail at a
// directory boundary.
func near(f Finding, l Label) bool {
	if f.File == "" || !sameFile(f.File, cleanPath(l.File)) {
		return false
	}
	return slices.ContainsFunc(append([]int{l.Line}, l.Also...), func(line int) bool {
		return max(f.Line-line, line-f.Line) <= _window
	})
}

// lines is l's line then its also lines, comma-separated: where the judge is told the defect sits.
func (l Label) lines() string {
	s := strconv.Itoa(l.Line)
	for _, a := range l.Also {
		s += "," + strconv.Itoa(a)
	}
	return s
}

func sameFile(a, b string) bool {
	return a == b || strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a)
}

// GradeReview scores report against labels per design.md §5. Every finding near a label is put to
// the judge, which decides the match. Recall is the share of mustFind labels any finding matched.
// A finding that matches no label, of any severity, is put to the judge against every mustFind label
// it is not near; one that names a labelled defect away from its label is no extra, no implausible
// finding and no true positive. Every other one is put to the judge for plausibility against diff:
// plausible, it is an extra; otherwise it is implausible (decision review-extras-net). Precision is
// over Critical and Important findings: matched to mustFind near its label is a true positive;
// otherwise matched to knownFalse, or implausible, a false positive; an extra is in neither count. Nothing to find gives recall 1,
// and no counted finding precision 1. severity_agreement is the share of mustFind matches whose
// finding repeats a labelled severity; duplicates counts every match to a label beyond its first.
func GradeReview(ctx context.Context, report, diff string, labels ReviewLabels, j Judge) (Review, error) {
	var r Review
	same := func(f Finding, l Label) (bool, error) {
		ok, u, err := j.Same(ctx, fmt.Sprintf("%s:%s: %s", l.File, l.lines(), l.Summary), f.Text)
		addUsage(&r.JudgeUsage, u)
		return ok, err
	}
	mustHits := make([]int, len(labels.MustFind))
	falseHits := make([]int, len(labels.KnownFalse))
	var tp, fp, agreed, rated int
	for _, f := range ParseReview(report) {
		var isTP, isFalse bool
		for i, l := range labels.MustFind {
			if !near(f, l) {
				continue
			}
			ok, err := same(f, l)
			if err != nil {
				return r, err
			}
			if !ok {
				continue
			}
			isTP = true
			mustHits[i]++
			if l.Severity != "" {
				rated++
				if strings.EqualFold(l.Severity, f.Severity) {
					agreed++
				}
			}
		}
		for i, l := range labels.KnownFalse {
			if !near(f, l) {
				continue
			}
			ok, err := same(f, l)
			if err != nil {
				return r, err
			}
			if ok {
				isFalse = true
				falseHits[i]++
			}
		}
		if isFalse {
			r.Metrics.KnownFalseHits++
		}
		counted := f.Severity != "Minor" // precision is over Critical and Important findings
		switch {
		case isTP:
			if counted {
				tp++
			}
		case isFalse:
			if counted {
				fp++
			}
		default:
			restated := false // f names a mustFind defect away from its label
			for _, l := range labels.MustFind {
				if near(f, l) {
					continue // already put to the judge above
				}
				ok, err := same(f, l)
				if err != nil {
					return r, err
				}
				if ok {
					restated = true
					break
				}
			}
			if restated {
				continue
			}
			ok, u, err := j.Plausible(ctx, diff, f.Text)
			addUsage(&r.JudgeUsage, u)
			if err != nil {
				return r, err
			}
			if ok {
				r.Metrics.Extras++
				continue
			}
			r.Metrics.Implausible++
			if counted {
				fp++
			}
		}
	}

	for _, n := range slices.Concat(mustHits, falseHits) {
		r.Metrics.Duplicates += max(n-1, 0)
	}
	found := 0
	for _, n := range mustHits {
		if n > 0 {
			found++
		}
	}
	m := &r.Metrics
	m.Recall = ratio(found, len(labels.MustFind), 1)
	m.Precision = ratio(tp, tp+fp, 1)
	m.SeverityAgreement = ratio(agreed, rated, 0)
	if m.Precision+m.Recall > 0 {
		m.F1 = 2 * m.Precision * m.Recall / (m.Precision + m.Recall)
	}
	return r, nil
}

// ratio is n/d, or ifNone when d is 0.
func ratio(n, d int, ifNone float64) float64 {
	if d == 0 {
		return ifNone
	}
	return float64(n) / float64(d)
}
