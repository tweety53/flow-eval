package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tweety53/flow-eval/internal/cases"
)

// Regrade grades every graded trial of the stored review run id again, against cfg.Cases — their
// labels as they stand now — and cfg.Judge, then rewrites runs/<id>.jsonl and its reports (decision
// regrade-from-stored-outputs). A trial's own metrics are kept; its review grade, score, pass and
// judge usage are replaced. The header takes the cases' current hashes and the judge's pin, and
// records cfg.FlowEvalSHA as Regraded. Each trial's report is read from its output.md, or, once that
// file is gone — an older run's directory was never committed — from the copy the run page carries,
// which the re-rendered page keeps; a copy the page cut or could not read is refused. Nothing is
// written unless every trial re-grades.
func Regrade(ctx context.Context, cfg Config, id string) error {
	h, lines, err := Load(cfg.RunsDir, id)
	if err != nil {
		return err
	}
	if h.Role != "review" {
		return fmt.Errorf("regrade %s: a %s run; only review runs are re-graded", id, h.Role)
	}
	var cs []cases.Case
	for _, cid := range slices.Sorted(maps.Keys(h.CaseHashes)) {
		i := slices.IndexFunc(cfg.Cases, func(c cases.Case) bool { return c.ID == cid })
		if i < 0 {
			return fmt.Errorf("regrade %s: no review case %q", id, cid)
		}
		cs = append(cs, cfg.Cases[i])
	}
	cfg.Role, cfg.Slot, cfg.Cases = h.Role, h.Slot, cs
	page, err := os.ReadFile(filepath.Join(cfg.RunsDir, id+".html"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for i := range lines {
		l := &lines[i]
		hadGradeErr := strings.HasPrefix(l.Error, "grade: ")
		if l.InfraError || (l.Review == nil && !hadGradeErr) {
			continue // never graded: there is no grade to replace
		}
		c := cs[slices.IndexFunc(cs, func(c cases.Case) bool { return c.ID == l.Case })]
		art := filepath.Join(cfg.RunsDir, id, l.Case, fmt.Sprint(l.TrialNum))
		report, err := trialOutput(art, page, l)
		if err != nil {
			return fmt.Errorf("regrade %s: %w", id, err)
		}
		judge, err := gradeTrial(ctx, cfg, nil, c, report, art, l)
		var jerr error
		l.Judge, jerr = judgeMetrics(cfg, judge)
		if err = errors.Join(err, jerr); err != nil {
			return fmt.Errorf("regrade %s: %s trial %d: %w", id, l.Case, l.TrialNum, err)
		}
		if hadGradeErr { // the old grade's failure is gone; a sandbox-stop failure runTrial noted stays
			_, stop, stopped := strings.CutLast(l.Error, _stopSandbox)
			l.Error = ""
			if stopped {
				l.Error = _stopSandbox + stop
			}
		}
	}
	if h.CaseSetHash, h.CaseHashes, err = caseHashes(cs); err != nil {
		return err
	}
	h.Judge, h.Regraded = cfg.JudgePin, cfg.FlowEvalSHA
	var b bytes.Buffer
	err = writeJSONLine(&b, h)
	for _, l := range lines {
		err = errors.Join(err, writeJSONLine(&b, l))
	}
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(cfg.RunsDir, id+".jsonl"), b.Bytes()); err != nil {
		return err
	}
	return writeReports(cfg, id, h, lines)
}

// trialOutput returns the trial's output.md from art, or else the copy page carries for it, refused
// when the page cut it or could not read it.
func trialOutput(art string, page []byte, l *Line) (string, error) {
	out, err := os.ReadFile(filepath.Join(art, "output.md"))
	if err == nil {
		return string(out), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	s, ok := pageArtifact(string(page), l.Case, l.TrialNum, "output.md")
	switch {
	case !ok:
		return "", fmt.Errorf("%s trial %d: no %s and no output.md on the run page", l.Case, l.TrialNum,
			filepath.Join(art, "output.md"))
	case strings.HasPrefix(s, _unreadable), strings.Contains(s, _cutNote):
		return "", fmt.Errorf("%s trial %d: no %s, and the run page's copy is not the whole report", l.Case,
			l.TrialNum, filepath.Join(art, "output.md"))
	}
	return s, nil
}
