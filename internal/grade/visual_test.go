package grade

import (
	"math"
	"slices"
	"testing"
)

func TestParseVisualVerdict(t *testing.T) {
	tests := []struct {
		name        string
		report      string
		wantVerdict string
		wantDefects []string
	}{
		{
			name:        "defect lines and a closing verdict",
			report:      "Compared both.\n- defect: button fill is grey, not blue\n  - defect: title 2px low\nverdict: mismatch\n",
			wantVerdict: "mismatch",
			wantDefects: []string{"button fill is grey, not blue", "title 2px low"},
		},
		{
			name:        "bold and numbered defect lines",
			report:      "- **defect:** icon tint\n1. defect: card radius\n2) **defect**: gap\nverdict: mismatch\n",
			wantVerdict: "mismatch",
			wantDefects: []string{"icon tint", "card radius", "gap"},
		},
		{
			name:        "the last verdict line wins",
			report:      "At first glance verdict: mismatch\nverdict: mismatch\nOn measuring:\n**verdict: match**",
			wantVerdict: "match",
		},
		{
			name:        "no verdict line",
			report:      "- defect: spacing\nI could not decide.",
			wantDefects: []string{"spacing"},
		},
		{
			name:   "a verdict naming neither value is no verdict",
			report: "verdict: unsure",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict, defects := ParseVisual(tt.report)
			if verdict != tt.wantVerdict || !slices.Equal(defects, tt.wantDefects) {
				t.Errorf("ParseVisual = %q, %q; want %q, %q", verdict, defects, tt.wantVerdict, tt.wantDefects)
			}
		})
	}
	t.Run("a missing verdict is scored wrong", func(t *testing.T) {
		got, err := GradeVisual(t.Context(), "looks the same", VisualLabels{Verdict: "match"}, &fakeJudge{})
		if err != nil {
			t.Fatal(err)
		}
		if got.Metrics.Correct {
			t.Error("a report with no verdict line was scored correct")
		}
	})
}

func TestFalsePassCounted(t *testing.T) {
	tests := []struct {
		name          string
		labelled      string
		reported      string
		wantCorrect   bool
		wantFalsePass bool
	}{
		{"match on a mismatch case is a false pass", "mismatch", "match", false, true},
		{"mismatch on a match case is wrong but no false pass", "match", "mismatch", false, false},
		{"mismatch on a mismatch case is correct", "mismatch", "mismatch", true, false},
		{"match on a match case is correct", "match", "match", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := VisualLabels{Verdict: tt.labelled}
			if tt.labelled == "mismatch" {
				labels.Defects = []VisualDefect{{Summary: "tag:A"}}
			}
			got, err := GradeVisual(t.Context(), "verdict: "+tt.reported, labels, &fakeJudge{})
			if err != nil {
				t.Fatal(err)
			}
			if got.Metrics.Correct != tt.wantCorrect || got.Metrics.FalsePass != tt.wantFalsePass {
				t.Errorf("correct %v, false_pass %v; want %v, %v",
					got.Metrics.Correct, got.Metrics.FalsePass, tt.wantCorrect, tt.wantFalsePass)
			}
		})
	}
}

func TestLocalizationWithFakeJudge(t *testing.T) {
	labels := VisualLabels{Verdict: "mismatch", Defects: []VisualDefect{
		{Summary: "fill tag:A"}, {Summary: "radius tag:B"}, {Summary: "gap tag:C"},
	}}
	report := "- defect: the fill differs tag:A\n- defect: gap too wide tag:C\n- defect: gap again tag:C\n" +
		"- defect: an unlabelled shadow\nverdict: mismatch\n"
	j := &fakeJudge{}
	got, err := GradeVisual(t.Context(), report, labels, j)
	if err != nil {
		t.Fatal(err)
	}
	if want := 2.0 / 3; math.Abs(got.Metrics.Localization-want) > 1e-9 {
		t.Errorf("localization = %v, want %v", got.Metrics.Localization, want)
	}
	if calls := int64(len(j.sameAsked)); calls == 0 || got.JudgeUsage.Input != calls*_callUsage.Input {
		t.Errorf("JudgeUsage.Input = %d over %d judge calls, want %d each", got.JudgeUsage.Input, calls, _callUsage.Input)
	}
}
