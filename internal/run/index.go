package run

import (
	"bytes"
	"cmp"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tweety53/flow-eval/internal/cases"
)

// indexRun is one run as the index lists it.
type indexRun struct {
	ID             string
	H              Header
	S              summary
	Cases          int
	Trials, Valid  int
	Passes, Infra  int
	Cost, JudgeUSD float64
	// WallMin, WallAvg and WallMax are its valid trials' wall-clock in seconds; nil with none.
	WallMin, WallAvg, WallMax *float64
	HasPage                   bool // runs from before the HTML page have none
	lines                     []Line
}

// indexRole is one role's runs, newest first.
type indexRole struct {
	Role string
	Runs []indexRun
}

// _indexRoles is the order the index lists roles in.
var _indexRoles = []string{"impl", "review", "visual"}

// Baseline is baselines/<role>.json: the run a role's candidates are compared against.
type Baseline struct {
	RunID string    `json:"runId"`
	SetAt time.Time `json:"setAt"`
}

// BaselineID returns the run id baselinesDir records for role.
func BaselineID(baselinesDir, role string) (string, error) {
	data, err := os.ReadFile(filepath.Join(baselinesDir, role+".json"))
	if err != nil {
		return "", fmt.Errorf("baseline for %s: %w", role, err)
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return "", fmt.Errorf("baseline for %s: %w", role, err)
	}
	if b.RunID == "" {
		return "", fmt.Errorf("baseline for %s: no runId", role)
	}
	return b.RunID, nil
}

// SetBaseline records the run id, loaded from runsDir, as its role's baseline in baselinesDir, and
// returns the role. Repainting runsDir's index and pages by the new baseline is Repaint.
func SetBaseline(runsDir, baselinesDir, id string, now time.Time) (string, error) {
	h, _, err := Load(runsDir, id)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(Baseline{RunID: id, SetAt: now.UTC()})
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(baselinesDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(baselinesDir, h.Role+".json"), append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return h.Role, nil
}

// Comparable returns nil when the candidate run candID can be compared with the baseline run baseID
// (design.md §7), else why not: another role or slot, another case set (naming the differing cases),
// another k, or a case with fewer than k valid trials on either side. compare refuses such a pair with
// exit 2, and Repaint never paints its rows as beating the baseline. A differing prompt pin is
// comparable: compare warns on it instead.
func Comparable(candID string, cand Header, candLines []Line, baseID string, base Header, baseLines []Line) error {
	if cand.Role != base.Role || cand.Slot != base.Slot {
		return fmt.Errorf("compare %s with %s: role %s/%s against %s/%s", candID, baseID,
			cand.Role, cand.Slot, base.Role, base.Slot)
	}
	if cand.CaseSetHash != base.CaseSetHash {
		all := maps.Clone(cand.CaseHashes)
		maps.Copy(all, base.CaseHashes)
		var differ []string
		for _, id := range slices.Sorted(maps.Keys(all)) {
			if cand.CaseHashes[id] != base.CaseHashes[id] {
				differ = append(differ, id)
			}
		}
		return fmt.Errorf("compare %s with %s: case sets differ: %s", candID, baseID, strings.Join(differ, ", "))
	}
	if cand.K != base.K {
		return fmt.Errorf("compare %s with %s: k %d against %d; re-run the candidate at k=%d",
			candID, baseID, cand.K, base.K, base.K)
	}
	for _, r := range []struct {
		id    string
		h     Header
		lines []Line
	}{{candID, cand, candLines}, {baseID, base, baseLines}} {
		valid := map[string]int{}
		for _, l := range r.lines {
			if !l.InfraError {
				valid[l.Case]++
			}
		}
		var short []string
		for _, id := range slices.Sorted(maps.Keys(r.h.CaseHashes)) {
			if valid[id] < r.h.K {
				short = append(short, fmt.Sprintf("%s (%d of %d)", id, valid[id], r.h.K))
			}
		}
		if len(short) > 0 {
			return fmt.Errorf("run %s: cases with fewer than k valid trials: %s", r.id, strings.Join(short, ", "))
		}
	}
	return nil
}

// Repaint rewrites runsDir/index.html from every run file in runsDir, and re-renders every run page
// from its run file, so each case and run row is painted by its outcome and by the role's baseline in
// baselinesDir. A run file it cannot read is listed with its error and keeps its page, never failing
// the run that writes the index; a role whose baseline is missing, unreadable or names no run here
// has no baseline. A page it cannot render or replace keeps its old content and never stops the other
// pages or the index: pages is every such page's error joined, err the index's.
func Repaint(runsDir, baselinesDir string) (pages, err error) {
	entries, err := os.ReadDir(runsDir) // never a glob: the runs path may hold pattern characters
	if err != nil {
		return nil, err
	}
	byRole := map[string][]indexRun{}
	var unreadable, pageErrs []error
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".jsonl")
		if !ok || e.IsDir() {
			continue
		}
		h, lines, err := Load(runsDir, id)
		if err != nil {
			unreadable = append(unreadable, err)
			continue
		}
		r := newIndexRun(id, h, lines)
		_, err = os.Stat(filepath.Join(runsDir, id+".html"))
		r.HasPage = err == nil
		byRole[h.Role] = append(byRole[h.Role], r)
	}
	for role, runs := range byRole {
		bi := -1
		if baseID, err := BaselineID(baselinesDir, role); err == nil {
			bi = slices.IndexFunc(runs, func(r indexRun) bool { return r.ID == baseID })
		}
		for i := range runs {
			r := &runs[i]
			var base *summary
			if bi >= 0 && i != bi {
				b := &runs[bi]
				if Comparable(r.ID, r.H, r.lines, b.ID, b.H, b.lines) == nil {
					base = &b.S // painting sets classes only, never the scores compared
				}
			}
			r.S.paint(base)
			if !r.HasPage {
				continue
			}
			html, err := page(runsDir, r.ID, r.H, r.S, r.lines)
			if err == nil {
				err = writeAtomic(filepath.Join(runsDir, r.ID+".html"), html)
			}
			if err != nil {
				pageErrs = append(pageErrs, fmt.Errorf("run page %s: %w", r.ID, err))
			}
		}
	}
	var roles []indexRole
	// The known roles first, in their order, then any other role a run file names, by name.
	others := slices.Sorted(maps.Keys(byRole))
	others = slices.DeleteFunc(others, func(r string) bool { return slices.Contains(_indexRoles, r) })
	for _, role := range slices.Concat(_indexRoles, others) {
		runs := byRole[role]
		if len(runs) == 0 {
			continue
		}
		slices.SortFunc(runs, func(a, b indexRun) int {
			return cmp.Or(b.H.Started.Compare(a.H.Started), strings.Compare(b.ID, a.ID))
		})
		roles = append(roles, indexRole{role, runs})
	}
	var b bytes.Buffer
	if err := _index.Execute(&b, struct {
		Roles      []indexRole
		Unreadable []error
	}{roles, unreadable}); err != nil {
		return errors.Join(pageErrs...), fmt.Errorf("runs index: %w", err)
	}
	return errors.Join(pageErrs...), writeAtomic(filepath.Join(runsDir, "index.html"), b.Bytes())
}

// writeAtomic replaces path with data through a rename, so two runs sharing a runs directory never
// leave a torn file and a reader never sees a half-written one.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // a no-op once renamed
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func newIndexRun(id string, h Header, lines []Line) indexRun {
	// Every case the run was given, whether or not a trial line was written, and any case a trial
	// line names that the header does not.
	ids := slices.Sorted(maps.Keys(h.CaseHashes))
	for _, l := range lines {
		if !slices.Contains(ids, l.Case) {
			ids = append(ids, l.Case)
		}
	}
	var cs []cases.Case
	for _, c := range ids {
		cs = append(cs, cases.Case{ID: c})
	}
	r := indexRun{ID: id, H: h, S: summarize(h, cs, lines), Cases: len(cs), Trials: len(lines), lines: lines}
	for _, row := range r.S.Rows {
		r.Valid += row.Valid
		r.Passes += row.Passes
		r.Cost += row.Cost
		r.JudgeUSD += row.JudgeCost
	}
	r.Infra = len(r.S.Infra)
	var walls []float64
	for _, l := range lines {
		if !l.InfraError {
			walls = append(walls, float64(l.WallMS)/1000)
		}
	}
	if len(walls) > 0 {
		lo, hi := slices.Min(walls), slices.Max(walls)
		var sum float64
		for _, w := range walls {
			sum += w
		}
		avg := sum / float64(len(walls))
		r.WallMin, r.WallAvg, r.WallMax = &lo, &avg, &hi
	}
	return r
}

var _index = template.Must(template.New("index").Funcs(template.FuncMap{"oneDecimal": oneDecimal}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>flow-eval runs</title>
<style>` + _css + `</style>
</head>
<body>
<h1>flow-eval runs</h1>
{{- if .Roles}}
<nav><p>{{range $i, $r := .Roles}}{{if $i}} · {{end}}<a href="#{{$r.Role}}">{{$r.Role}}</a> ({{len $r.Runs}}){{end}}</p></nav>
` + _legend + `
{{- else}}
<p class="muted">No run recorded yet.</p>
{{- end}}
{{- range .Roles}}

<h2 id="{{.Role}}">{{.Role}}</h2>
<div class="table"><table>
<tr><th>run</th><th>started</th><th>harness</th><th>model</th><th>served</th><th>effort</th><th>slot</th><th>judge</th><th>k</th><th>cases</th><th>role score</th><th>valid/trials</th><th>passes</th><th>infra errors</th><th>cost</th><th>judge cost</th><th>wall min s</th><th>wall avg s</th><th>wall max s</th><th>flow-eval</th></tr>
{{- range .Runs}}
<tr{{with .S.Class}} class="{{.}}"{{end}}><td>{{if .HasPage}}<a href="./{{.ID}}.html">{{.ID}}</a>{{else}}{{.ID}} <span class="muted">(no page)</span>{{end}}</td><td>{{.H.Started.Format "2006-01-02 15:04"}}</td><td>{{.H.Harness}}</td><td>{{.H.Model}}</td><td>{{or .S.Served "–"}}</td><td>{{.H.Effort}}</td><td>{{.H.Slot}}</td><td>{{.H.Judge}}</td><td>{{.H.K}}</td><td>{{.Cases}}</td><td>{{if .S.HasScore}}{{printf "%.1f" .S.Score}}{{else}}–{{end}}</td><td>{{.Valid}}/{{.Trials}}</td><td>{{.Passes}}</td><td>{{.Infra}}</td><td>{{printf "$%.4f" .Cost}}</td><td>{{printf "$%.4f" .JudgeUSD}}</td><td>{{oneDecimal .WallMin}}</td><td>{{oneDecimal .WallAvg}}</td><td>{{oneDecimal .WallMax}}</td><td>{{printf "%.7s" .H.FlowEvalSHA}}</td></tr>
{{- end}}
</table></div>
{{- end}}
{{- if .Unreadable}}

<h2>Unreadable run files</h2>
<ul>
{{- range .Unreadable}}
<li class="error">{{.}}</li>
{{- end}}
</ul>
{{- end}}
</body>
</html>
`))
