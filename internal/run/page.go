package run

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"unicode/utf8"

	"encoding/json/jsontext"
	json "encoding/json/v2"

	"github.com/tweety53/flow-eval/internal/metrics"
)

// pageTrial is one trial as the HTML page shows it.
type pageTrial struct {
	Line
	Metrics, Judge []metric
	Transcript     string // the transcript's path relative to the page
	Artifacts      []artifact
}

// pageCase is one case's trials, in trial order.
type pageCase struct {
	ID     string
	Trials []pageTrial
}

// metric is one field of a trial's JSONL line; an object or array is shown as its JSON.
type metric struct{ Name, Value string }

// artifact is one of a trial's files, shown inline.
type artifact struct{ Name, Body string }

// _pageArtifacts are the trial files the page carries inline, in this order; a role that writes
// none of them shows none.
var _pageArtifacts = []string{"output.md", "verify.log", "work.patch"}

// _artifactCap is how much of one artifact the page carries; a longer one is cut, naming its file.
const _artifactCap = 256 << 10

// page renders a run as one self-contained HTML page: no script and no external asset, every value
// escaped by html/template. Its cases are s's rows, in their order. It reads each trial's artifacts
// from runsDir/<id>/<case>/<trial>/; an artifact it cannot read is shown as its read error, never
// failing the run.
func page(runsDir, id string, h Header, s summary, lines []Line) ([]byte, error) {
	var byCase []pageCase
	for _, c := range s.Rows {
		var own []Line
		for _, l := range lines {
			if l.Case == c.ID {
				own = append(own, l)
			}
		}
		slices.SortFunc(own, func(a, b Line) int { return a.TrialNum - b.TrialNum })
		pc := pageCase{ID: c.ID}
		for _, l := range own {
			t, err := newPageTrial(runsDir, id, l)
			if err != nil {
				return nil, err
			}
			pc.Trials = append(pc.Trials, t)
		}
		byCase = append(byCase, pc)
	}
	var b bytes.Buffer
	err := _page.Execute(&b, struct {
		ID    string
		H     Header
		S     summary
		Cases []pageCase
	}{id, h, s, byCase})
	if err != nil {
		return nil, fmt.Errorf("run %s: html page: %w", id, err)
	}
	return b.Bytes(), nil
}

func newPageTrial(runsDir, id string, l Line) (pageTrial, error) {
	rel := path.Join(id, l.Case, fmt.Sprint(l.TrialNum))
	t := pageTrial{Line: l, Transcript: path.Join(rel, "transcript.jsonl")}
	var err error
	if t.Metrics, err = metricsOf(l.Trial); err != nil {
		return pageTrial{}, err
	}
	if l.Judge != nil {
		if t.Judge, err = metricsOf(*l.Judge); err != nil {
			return pageTrial{}, err
		}
	}
	for _, name := range _pageArtifacts {
		body, err := os.ReadFile(filepath.Join(runsDir, filepath.FromSlash(rel), name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil: // the trial already records why it could not write the file
			body = []byte("unreadable: " + err.Error())
		case len(body) > _artifactCap:
			n := _artifactCap
			for n > 0 && !utf8.RuneStart(body[n]) { // never cut inside a character
				n--
			}
			body = fmt.Appendf(body[:n:n], "\n… cut at %d KiB; the whole file is %s", _artifactCap>>10, path.Join(rel, name))
		}
		t.Artifacts = append(t.Artifacts, artifact{Name: name, Body: string(body)})
	}
	return t, nil
}

// metricsOf lists every field of a trial's JSONL record, by name, as the record writes it: a
// number in full, a string unquoted, null as "–", an object or array as its JSON.
func metricsOf(m metrics.Trial) ([]metric, error) {
	data, err := json.Marshal(m, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	var out []metric
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		v := fields[name]
		switch v.Kind() {
		case '"':
			var str string
			if err := json.Unmarshal(v, &str); err != nil {
				return nil, err
			}
			out = append(out, metric{name, str})
		case 'n':
			out = append(out, metric{name, "–"})
		default:
			out = append(out, metric{name, string(v)})
		}
	}
	return out, nil
}

// _legend names the row colours, above the run page's cases and the index's runs.
const _legend = `<p class="legend"><span class="row-ok">every valid trial passed</span><span class="row-partial">some passed</span><span class="row-fail">none passed</span><span class="row-better">beats the role's baseline run</span></p>`

// _css styles both the run pages and the index.
const _css = `:root { --bg: #fff; --fg: #1d1d1f; --muted: #6e6e73; --line: #d2d2d7; --panel: #f5f5f7; --pass: #1a7f37; --fail: #cf222e; --warn: #9a6700;
  --row-ok: #dafbe1; --row-partial: #fff8c5; --row-fail: #ffebe9; --row-better: #ddf4ff; }
@media (prefers-color-scheme: dark) {
  :root { --bg: #161618; --fg: #f5f5f7; --muted: #a1a1a6; --line: #3a3a3c; --panel: #222225; --pass: #3fb950; --fail: #f85149; --warn: #d29922;
    --row-ok: #1b3022; --row-partial: #352f17; --row-fail: #3d1d20; --row-better: #172c45; }
}
body { margin: 0 auto; max-width: 1200px; padding: 24px 16px; background: var(--bg); color: var(--fg);
  font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
h1 { font-size: 20px; margin: 0 0 16px; word-break: break-all; }
h2 { font-size: 16px; margin: 32px 0 8px; }
h3 { font-size: 14px; margin: 0; }
dl.header { display: grid; grid-template-columns: max-content 1fr; gap: 4px 16px; margin: 0; }
dt { color: var(--muted); }
dd { margin: 0; font-family: ui-monospace, Menlo, monospace; word-break: break-all; }
.score { font-size: 28px; font-weight: 600; }
.muted { color: var(--muted); }
.table { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; }
th, td { border-bottom: 1px solid var(--line); padding: 6px 8px; text-align: left; white-space: nowrap; }
th { color: var(--muted); font-weight: 500; }
.row-ok { background: var(--row-ok); }
.row-partial { background: var(--row-partial); }
.row-fail { background: var(--row-fail); }
.row-better { background: var(--row-better); }
.legend span { padding: 2px 8px; border-radius: 4px; margin-right: 4px; }
.trial { border: 1px solid var(--line); border-radius: 8px; padding: 12px 16px; margin: 12px 0; }
.trial.pass { border-left: 4px solid var(--pass); }
.trial.fail { border-left: 4px solid var(--fail); }
.trial.infra { border-left: 4px solid var(--warn); }
.verdict { font-weight: 600; }
.pass .verdict { color: var(--pass); }
.fail .verdict { color: var(--fail); }
.infra .verdict { color: var(--warn); }
td.gain { color: var(--pass); font-weight: 600; }
td.loss { color: var(--fail); font-weight: 600; }
.error { color: var(--fail); white-space: pre-wrap; font-family: ui-monospace, Menlo, monospace; }
dl.metrics { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 2px 16px; margin: 8px 0; }
dl.metrics div { display: flex; gap: 8px; min-width: 0; }
dl.metrics dd { overflow-wrap: anywhere; }
details { margin: 8px 0; }
summary { cursor: pointer; color: var(--muted); }
pre { background: var(--panel); padding: 12px; border-radius: 6px; overflow-x: auto; max-height: 600px; margin: 4px 0; }
nav { margin: 0 0 16px; }
nav ul { margin: 4px 0; padding-left: 20px; }
a { color: inherit; }
`

var _page = template.Must(template.New("page").Funcs(template.FuncMap{
	"yesNo": yesNo, "money": money, "oneDecimal": oneDecimal,
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Run {{.ID}}</title>
<style>` + _css + `</style>
</head>
<body>
<nav><a href="index.html">← all runs</a></nav>
<h1>Run {{.ID}}</h1>
<dl class="header">
<dt>role</dt><dd>{{.H.Role}}</dd>
<dt>harness</dt><dd>{{.H.Harness}}</dd>
<dt>model</dt><dd>{{.H.Model}}</dd>
<dt>served</dt><dd>{{or .S.Served "–"}}</dd>
<dt>effort</dt><dd>{{.H.Effort}}</dd>
<dt>k</dt><dd>{{.H.K}}</dd>
{{- if .H.Slot}}
<dt>slot</dt><dd>{{.H.Slot}}</dd>
{{- end}}
<dt>judge</dt><dd>{{.H.Judge}}</dd>
<dt>prompt pin</dt><dd>{{.H.PromptPin}}</dd>
<dt>case set</dt><dd>{{.H.CaseSetHash}}</dd>
<dt>flow-eval</dt><dd>{{.H.FlowEvalSHA}}</dd>
<dt>started</dt><dd>{{.H.Started.Format "2006-01-02T15:04:05Z07:00"}}</dd>
</dl>

<nav><strong>Contents</strong>
<ul>
{{- range .Cases}}
<li><a href="#case-{{.ID}}">{{.ID}}</a>{{range .Trials}} · <a href="#trial-{{.Case}}-{{.TrialNum}}">trial {{.TrialNum}}</a>{{end}}</li>
{{- end}}
</ul></nav>

<h2>Role score</h2>
{{- if .S.HasScore}}
<p><span class="score">{{printf "%.1f" .S.Score}}</span> <span class="muted">{{.S.How}}, infra errors excluded</span></p>
{{- else}}
<p class="muted">No valid trial: no role score.</p>
{{- end}}

<h2>Cases</h2>
` + _legend + `
<div class="table"><table>
<tr><th>case</th><th>valid/k</th><th>passes</th><th>pass^k</th><th>score</th><th>stddev</th><th>cost/pass</th><th>cost</th><th>mean wall s</th><th>tokens</th><th>judge cost</th></tr>
{{- range .S.Rows}}
<tr id="case-{{.ID}}"{{with .Class}} class="{{.}}"{{end}}><td>{{.ID}}</td><td>{{.Valid}}/{{.K}}</td><td>{{.Passes}}</td><td>{{yesNo .PassK}}</td><td>{{printf "%.1f" .Score}}</td><td>{{printf "%.1f" .StdDev}}</td><td>{{money .CostPerPass}}</td><td>{{printf "$%.4f" .Cost}}</td><td>{{oneDecimal .MeanWallS}}</td><td>{{.Tokens}}</td><td>{{printf "$%.4f" .JudgeCost}}</td></tr>
{{- end}}
</table></div>
{{- if .S.Infra}}

<h2>Infra errors (not scored)</h2>
<ul>
{{- range .S.Infra}}
<li>{{.Case}} trial {{.TrialNum}}: <span class="error">{{.Error}}</span></li>
{{- end}}
</ul>
{{- end}}

<h2>Trials</h2>
{{- range .Cases}}{{range .Trials}}
<section id="trial-{{.Case}}-{{.TrialNum}}" class="trial {{if .InfraError}}infra{{else if .Passed}}pass{{else}}fail{{end}}">
<h3>{{.Case}} · trial {{.TrialNum}} · <span class="verdict">{{if .InfraError}}infra error{{else if .Passed}}pass{{else}}fail{{end}}</span> · score {{printf "%.1f" .Score}}</h3>
{{- if .Error}}
<p class="error">{{.Error}}</p>
{{- end}}
<dl class="metrics">
{{- range .Metrics}}
<div><dt>{{.Name}}</dt><dd>{{.Value}}</dd></div>
{{- end}}
</dl>
{{- if .Judge}}
<details><summary>judge usage (not charged to the trial)</summary>
<dl class="metrics">
{{- range .Judge}}
<div><dt>{{.Name}}</dt><dd>{{.Value}}</dd></div>
{{- end}}
</dl>
</details>
{{- end}}
{{- range .Artifacts}}
<details><summary>{{.Name}}</summary><pre>{{.Body}}</pre></details>
{{- end}}
<p class="muted">transcript: {{.Transcript}}</p>
</section>
{{- end}}{{end}}
</body>
</html>
`))
