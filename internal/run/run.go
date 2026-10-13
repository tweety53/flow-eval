// Package run executes one role on its case set for k trials each, every trial in a fresh sandbox,
// and records the run as runs/<id>.jsonl, runs/<id>.md and the raw per-trial artifacts under
// runs/<id>/<case>/<trial>/ (design.md §1, §8).
package run

import (
	"archive/tar"
	"bufio"
	"cmp"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tweety53/flow-eval/internal/cases"
	"github.com/tweety53/flow-eval/internal/grade"
	"github.com/tweety53/flow-eval/internal/harness"
	"github.com/tweety53/flow-eval/internal/metrics"
	"github.com/tweety53/flow-eval/internal/prompts"
	"github.com/tweety53/flow-eval/internal/sandbox"
)

// Timeouts bound one agent command per role, and separately the grading of its work; flow's own
// review ceiling is 15 minutes.
var Timeouts = map[string]time.Duration{
	"impl":   45 * time.Minute,
	"review": 15 * time.Minute,
	"visual": 10 * time.Minute,
}

// _promptFile is where the rendered prompt lands in the container; the harness's Command hands it to
// the CLI.
const _promptFile = "/case/prompt.md"

// _inputs lists, per role, the case files a trial's /case receives. Ground truth (heldout.patch,
// reference.stat, labels.json) is never among them.
var _inputs = map[string][]string{
	"impl":   {"task.md", "context.md"},
	"review": {"diff.patch"},
	"visual": {"mockup.png", "capture.png"},
}

// Sandbox is one started trial container.
type Sandbox interface {
	CopyIn(ctx context.Context, tarPath, dir string) error
	Exec(ctx context.Context, cmd ...string) (sandbox.Result, error)
	EgressDenied(ctx context.Context) (int, error)
	Stop(ctx context.Context) error
}

var _ Sandbox = (*sandbox.Trial)(nil)

// Docker starts trial sandboxes and warms a repository's Gradle cache.
type Docker interface {
	Start(ctx context.Context, spec sandbox.TrialSpec) (Sandbox, error)
	Warm(ctx context.Context, image, repo, snapshotTar string) error
}

// DockerHost is the Docker that drives the host's docker CLI through internal/sandbox.
type DockerHost struct{ CLI sandbox.Runner }

// Start implements Docker.
func (d DockerHost) Start(ctx context.Context, spec sandbox.TrialSpec) (Sandbox, error) {
	t, err := sandbox.Start(ctx, d.CLI, spec)
	if err != nil {
		return nil, err // never a nil *Trial inside a non-nil Sandbox
	}
	return t, nil
}

// Warm implements Docker.
func (d DockerHost) Warm(ctx context.Context, image, repo, snapshotTar string) error {
	return sandbox.WarmGradle(ctx, d.CLI, image, repo, snapshotTar)
}

// Config is one run.
type Config struct {
	Role, Model, Effort, Slot string
	Harness                   harness.Harness
	K, Parallel               int
	Cases                     []cases.Case
	// ReposDir holds each impl or review case's source checkout as <ReposDir>/<source.repo>.
	ReposDir, PromptsDir, RunsDir string
	BaselinesDir                  string // holds baselines/<role>.json, read to paint rows beating the baseline
	RunnerImage, ProxyImage       string
	Credentials                   []string // environment variable names passed into each trial
	Mounts                        []string // the harness's own volume --mount values, into each trial
	FlowEvalSHA                   string
	Pricing                       metrics.Pricing
	Judge                         grade.Judge
	JudgePin                      grade.Pin // recorded in a review or visual run's header; its model prices the judge's usage
	// Warnings receives what does not fail the run: another run's page that could not be repainted.
	// nil discards them.
	Warnings io.Writer
}

// Header is the first line of runs/<id>.jsonl (design.md §8).
type Header struct {
	Role        string            `json:"role"`
	Harness     string            `json:"harness"`
	Model       string            `json:"model"`
	Effort      string            `json:"effort"`
	Slot        string            `json:"slot,omitempty"`
	K           int               `json:"k"`
	PromptPin   string            `json:"prompt_pin"`
	CaseSetHash string            `json:"case_set_hash"`
	CaseHashes  map[string]string `json:"case_hashes"` // case id → its own set hash
	FlowEvalSHA string            `json:"flow_eval_sha"`
	Started     time.Time         `json:"started"`
	// Judge is the judge pin a review or visual run is graded with; an impl run, and every run from
	// before the field, records none.
	Judge grade.Pin `json:"judge,omitzero"`
}

// Line is one trial line of runs/<id>.jsonl.
type Line struct {
	Case     string `json:"case"`
	TrialNum int    `json:"trial"` // 1-based
	// Error is what went wrong: with InfraError set, why the trial was not scored; else why the scored
	// trial failed (an impl grade's reason, or a failure after the agent produced output).
	Error string `json:"error,omitempty"`
	metrics.Trial
	// Judge is the grader's judge usage and cost, never part of the trial's own cost.
	Judge *metrics.Trial `json:"judge,omitzero"`
	// StandardsDropped names each `## standards` entry of a principles trial's snapshot that resolved
	// to no file inside its root, as flow reports and drops it.
	StandardsDropped []string `json:"standards_dropped,omitempty"`
}

// prepared is one case ready for its trials.
type prepared struct {
	c        cases.Case
	snapshot string // the snapshot tar; "" for visual
	inputs   string // the /case tar: the role's inputs, the rendered prompt and any agents rule standard
	prompt   string
	dropped  []string // the snapshot's `## standards` entries a principles trial could not resolve
}

// now is the clock a run id is stamped from; a var so tests can pin it.
var now = time.Now

// Run runs every case of cfg k times and returns the run id. Each source repository's Gradle cache
// is warmed before its first trial. A trial whose sandbox or agent fails before the agent produced
// output is recorded as an infra error; any later failure, grading included, is a scored fail
// carrying its reason. err is reserved for a run that cannot proceed at all, such as a review slot
// with no prompt or a review case with no must-find for cfg.Slot, refused before any trial.
func Run(ctx context.Context, cfg Config, docker Docker) (string, error) {
	timeout, ok := Timeouts[cfg.Role]
	if !ok || cfg.K < 1 || cfg.Parallel < 1 || len(cfg.Cases) == 0 {
		return "", fmt.Errorf("run: want a known role, k ≥ 1, parallel ≥ 1 and a case, got %q, %d, %d, %d cases",
			cfg.Role, cfg.K, cfg.Parallel, len(cfg.Cases))
	}
	if cfg.Role == "review" {
		if err := prompts.Vendored(cfg.PromptsDir, reviewTarget(cfg.Slot)); err != nil {
			return "", fmt.Errorf("run: slot %s has no review prompt: %w", cfg.Slot, err)
		}
		var unfound []string
		for _, c := range cfg.Cases {
			labels, err := slotLabels(c, cfg.Slot)
			if err != nil {
				return "", fmt.Errorf("run: case %s: %w", c.ID, err)
			}
			if len(labels.MustFind) == 0 {
				unfound = append(unfound, c.ID)
			}
		}
		if len(unfound) > 0 {
			return "", fmt.Errorf("run: each of these review cases has no must-find raised by slot %s: %s; leave them out with --cases",
				cfg.Slot, strings.Join(unfound, ", "))
		}
	}
	started := now().UTC()
	id := fmt.Sprintf("%s-%s-%s-%s-%s", started.Format("20060102T150405Z"), cfg.Role, cfg.Harness.Name(),
		cfg.Model, cfg.Effort)
	h, err := header(cfg, started)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "flow-eval-run-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	preps, err := prepare(ctx, cfg, docker, tmp)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(cfg.RunsDir, 0o755); err != nil {
		return "", err
	}
	// O_EXCL: a second run with the same id (same second, role, harness, model and effort) is refused
	// before any container starts, never overwriting the first run's record or sharing its sandboxes.
	out, err := os.OpenFile(filepath.Join(cfg.RunsDir, id+".jsonl"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if err := writeJSONLine(out, h); err != nil {
		return "", err
	}

	var (
		mu       sync.Mutex // guards lines and out
		lines    []Line
		writeErr error
		wg       sync.WaitGroup
	)
	sem := make(chan struct{}, cfg.Parallel) // one slot per concurrent trial
	n := 0
jobs:
	for _, p := range preps {
		for trial := 1; trial <= cfg.K; trial++ {
			n++
			sandboxID := fmt.Sprintf("%s-%d", strings.ToLower(id), n)
			art := filepath.Join(cfg.RunsDir, id, p.c.ID, fmt.Sprint(trial))
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
			}
			if ctx.Err() != nil {
				break jobs
			}
			wg.Go(func() {
				defer func() { <-sem }()
				l := runTrial(ctx, cfg, docker, p, sandboxID, art, timeout)
				l.Case, l.TrialNum, l.StandardsDropped = p.c.ID, trial, p.dropped
				mu.Lock()
				defer mu.Unlock()
				lines = append(lines, l)
				writeErr = errors.Join(writeErr, writeJSONLine(out, l))
			})
		}
	}
	wg.Wait()
	// The reports cover the trials recorded so far, so a cancelled run still gets them.
	if err := errors.Join(ctx.Err(), writeErr, writeReports(cfg, id, h, lines)); err != nil {
		return id, fmt.Errorf("run %s: %w", id, err)
	}
	return id, nil
}

// writeReports writes runs/<id>.md and runs/<id>.html from the run's trial lines, then rewrites
// runs/index.html and repaints every run page by the role's baseline. A page Repaint could not
// replace is a warning, never this run's error: this run's own page is already written.
func writeReports(cfg Config, id string, h Header, lines []Line) error {
	s := summarize(h, cfg.Cases, lines)
	if err := os.WriteFile(filepath.Join(cfg.RunsDir, id+".md"), []byte(report(id, h, s)), 0o644); err != nil {
		return err
	}
	html, err := page(cfg.RunsDir, id, h, s, lines)
	if err != nil {
		return err
	}
	// Written before the index, so a run whose index write fails still has its page; atomically, so a
	// concurrent run's Repaint replacing it never leaves a torn page.
	if err := writeAtomic(filepath.Join(cfg.RunsDir, id+".html"), html); err != nil {
		return err
	}
	pages, err := Repaint(cfg.RunsDir, cfg.BaselinesDir)
	if pages != nil && cfg.Warnings != nil {
		fmt.Fprintf(cfg.Warnings, "warning: run %s recorded, repaint failed: %v\n", id, pages)
	}
	return err
}

func header(cfg Config, started time.Time) (Header, error) {
	pin, err := os.ReadFile(filepath.Join(cfg.PromptsDir, "PIN"))
	if err != nil {
		return Header{}, fmt.Errorf("run: prompt pin: %w", err)
	}
	h := Header{
		Role: cfg.Role, Harness: cfg.Harness.Name(), Model: cfg.Model, Effort: cfg.Effort, K: cfg.K,
		PromptPin: strings.TrimSpace(string(pin)), CaseHashes: map[string]string{},
		FlowEvalSHA: cfg.FlowEvalSHA, Started: started,
	}
	if cfg.Role == "review" {
		h.Slot = cfg.Slot
	}
	if cfg.Role != "impl" { // impl calls no judge
		h.Judge = cfg.JudgePin
	}
	if h.CaseSetHash, err = cases.SetHash(cfg.Cases); err != nil {
		return Header{}, err
	}
	for _, c := range cfg.Cases {
		if h.CaseHashes[c.ID], err = cases.SetHash([]cases.Case{c}); err != nil {
			return Header{}, err
		}
	}
	return h, nil
}

// prepare snapshots each impl and review case's repository, warms each repository's Gradle cache
// once with its first snapshot, resolves a principles trial's standards from that snapshot, and
// renders each case's prompt and /case inputs.
func prepare(ctx context.Context, cfg Config, docker Docker, tmp string) ([]prepared, error) {
	warmed := map[string]bool{}
	var preps []prepared
	for i, c := range cfg.Cases {
		dir := filepath.Join(tmp, fmt.Sprint(i))
		if err := os.Mkdir(dir, 0o755); err != nil {
			return nil, err
		}
		p := prepared{c: c}
		if cfg.Role != "visual" {
			var err error
			p.snapshot, err = sandbox.Snapshot(ctx, filepath.Join(cfg.ReposDir, c.Source.Repo), c.Snapshot, dir)
			if err != nil {
				return nil, fmt.Errorf("case %s: %w", c.ID, err)
			}
			if !warmed[c.Source.Repo] {
				if err := docker.Warm(ctx, cfg.RunnerImage, c.Source.Repo, p.snapshot); err != nil {
					return nil, fmt.Errorf("case %s: %w", c.ID, err)
				}
				warmed[c.Source.Repo] = true
			}
		}
		var std []prompts.Standard
		if cfg.Role == "review" && cfg.Slot == "principles" {
			var err error
			std, p.dropped, err = prompts.Standards(cfg.PromptsDir, filepath.Join(dir, "tree"))
			if err != nil {
				return nil, fmt.Errorf("case %s: %w", c.ID, err)
			}
		}
		prompt, err := render(cfg, c, std)
		if err != nil {
			return nil, fmt.Errorf("case %s: %w", c.ID, err)
		}
		p.prompt = prompt
		p.inputs = filepath.Join(dir, "case.tar")
		if err := writeInputs(p.inputs, c.Dir, _inputs[cfg.Role], prompt, std); err != nil {
			return nil, fmt.Errorf("case %s: %w", c.ID, err)
		}
		preps = append(preps, p)
	}
	return preps, nil
}

// render renders c's prompt; std fills a review prompt's project standards, as flow joins them.
func render(cfg Config, c cases.Case, std []prompts.Standard) (string, error) {
	read := func(name string) (string, error) {
		b, err := os.ReadFile(filepath.Join(c.Dir, name))
		return string(b), err
	}
	in := map[string]string{}
	target := cfg.Role
	switch cfg.Role {
	case "impl":
		task, err := read("task.md")
		if err != nil {
			return "", err
		}
		ctxDoc, err := read("context.md")
		if err != nil {
			return "", err
		}
		in["Task"], in["Context"] = task, ctxDoc
	case "review":
		diff, err := read("diff.patch")
		if err != nil {
			return "", err
		}
		var paths []string
		for _, s := range std {
			paths = append(paths, s.Container)
		}
		in["Diff"], in["Standards"], target = diff, strings.Join(paths, ", "), reviewTarget(cfg.Slot)
	case "visual":
		in["Mockup"], in["Capture"] = "/case/mockup.png", "/case/capture.png"
	}
	return prompts.Render(cfg.PromptsDir, target, in)
}

// reviewTarget is the prompt a review trial of slot renders.
func reviewTarget(slot string) string { return "review-" + slot }

// writeInputs writes the tar extracted at the container's root: /case, holding the named files of
// caseDir and the prompt, and each standard std places outside /work — an agents rule — at its
// container path. A standard under /work is already there, in the snapshot.
func writeInputs(tarPath, caseDir string, names []string, prompt string, std []prompts.Standard) (err error) {
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	tw := tar.NewWriter(f)
	add := func(name string, data []byte) error {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Uname: "root", Gname: "root"}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	dirs := map[string]bool{}
	mkdir := func(name string) error {
		for _, d := range []string{path.Dir(name), name} { // two levels deep at most: case/, agents/rules/
			if d == "." || dirs[d] {
				continue
			}
			dirs[d] = true
			if err := tw.WriteHeader(&tar.Header{Name: d + "/", Typeflag: tar.TypeDir, Mode: 0o755, Uname: "root", Gname: "root"}); err != nil {
				return err
			}
		}
		return nil
	}
	if err := mkdir("case"); err != nil {
		return err
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(caseDir, name))
		if err != nil {
			return err
		}
		if err := add("case/"+name, data); err != nil {
			return err
		}
	}
	if err := add(strings.TrimPrefix(_promptFile, "/"), []byte(prompt)); err != nil {
		return err
	}
	for _, s := range std {
		if strings.HasPrefix(s.Container, "/work/") {
			continue
		}
		name := strings.TrimPrefix(s.Container, "/")
		data, err := os.ReadFile(s.Host)
		if err != nil {
			return err
		}
		if err := mkdir(path.Dir(name)); err != nil {
			return err
		}
		if err := add(name, data); err != nil {
			return err
		}
	}
	return tw.Close()
}

// runTrial runs one trial of p in a fresh sandbox and returns its line, Case and TrialNum unset.
func runTrial(ctx context.Context, cfg Config, docker Docker, p prepared, sandboxID, art string,
	timeout time.Duration,
) (l Line) {
	infra := func(err error) Line {
		return Line{Error: err.Error(), Trial: metrics.Trial{InfraError: true}}
	}
	if err := os.MkdirAll(art, 0o755); err != nil {
		return infra(err)
	}
	if err := os.WriteFile(filepath.Join(art, "prompt.md"), []byte(p.prompt), 0o644); err != nil {
		return infra(err)
	}
	spec := sandbox.TrialSpec{
		ID: sandboxID, NeedsDocker: p.c.NeedsDocker, Credentials: cfg.Credentials, Mounts: cfg.Mounts,
		RunnerImage: cfg.RunnerImage, ProxyImage: cfg.ProxyImage,
	}
	if p.snapshot != "" { // only a trial with a repository to build gets its Gradle cache
		spec.Repo = p.c.Source.Repo
	}
	sb, err := docker.Start(ctx, spec)
	if err != nil {
		return infra(fmt.Errorf("start sandbox: %w", err))
	}
	defer func() {
		if err := sb.Stop(ctx); err != nil {
			l.Error = strings.TrimSpace(l.Error + "; stop sandbox: " + err.Error())
		}
	}()
	if p.snapshot != "" {
		if err := sb.CopyIn(ctx, p.snapshot, "/work"); err != nil {
			return infra(err)
		}
	}
	if err := sb.CopyIn(ctx, p.inputs, "/"); err != nil {
		return infra(err)
	}

	argv, env := cfg.Harness.Command(_promptFile, cfg.Model, cfg.Effort, "/work")
	actx, cancel := context.WithTimeout(ctx, timeout)
	began := time.Now()
	res, execErr := sb.Exec(actx, append(append([]string{"env"}, env...), argv...)...)
	wall := time.Since(began)
	timedOut := errors.Is(execErr, context.DeadlineExceeded) && ctx.Err() == nil
	cancel()
	// Nothing the agent started may outlive it and touch the work the grader is about to read.
	if _, err := sb.Exec(ctx, "sh", "-c", "kill -KILL -1"); err != nil {
		return infra(fmt.Errorf("kill agent processes: %w", err))
	}
	if err := os.WriteFile(filepath.Join(art, "transcript.jsonl"), []byte(res.Stdout), 0o644); err != nil {
		return infra(err)
	}
	if res.Stdout == "" && !timedOut && (execErr != nil || res.Exit != 0) {
		return infra(errors.Join(fmt.Errorf("agent produced no output: exit %d: %s", res.Exit,
			strings.TrimSpace(res.Stderr)), execErr))
	}
	if execErr != nil && !timedOut {
		return infra(fmt.Errorf("agent: %w", execErr))
	}

	// The agent has run to its end or its timeout, with output: from here a failure of the transcript
	// or of grading is the trial's, scored as a fail carrying its reason, unless the run itself was
	// cancelled. A failure of the harness's own — pricing the trial's model, counting its egress
	// denials, writing an artifact — is still infra: the agent did not cause it.
	l = Line{}
	fail := func(err error) Line {
		if ctx.Err() != nil {
			return infra(err)
		}
		l.Passed, l.Score, l.Error = false, 0, err.Error()
		return l
	}
	u, err := cfg.Harness.Parse(strings.NewReader(res.Stdout))
	switch {
	case err != nil && timedOut:
		// A CLI killed at its timeout leaves a transcript cut off before its totals: the trial is a
		// scored timeout, not an infra error, or a candidate that always times out would look clean.
		// ponytail: its partial usage is dropped; sum the per-event usage if timeouts become common.
		u = harness.Usage{}
	case err != nil:
		return fail(fmt.Errorf("parse transcript: %w", err))
	}
	if err := os.WriteFile(filepath.Join(art, "output.md"), []byte(u.FinalText), 0o644); err != nil {
		return infra(err)
	}
	if l.Trial, err = metrics.FromUsage(u, cfg.Model, cfg.Pricing, wall); err != nil {
		return infra(err)
	}
	l.TimedOut = timedOut
	if l.EgressDenied, err = sb.EgressDenied(ctx); err != nil {
		return infra(fmt.Errorf("count egress denials: %w", err))
	}
	// Grading runs the agent's code — the held-out tests, the module suite — and judge calls, so it gets
	// its own deadline: a hung test is a failed trial, never a hung run.
	gctx, gcancel := context.WithTimeoutCause(ctx, timeout, fmt.Errorf("grading exceeded its %s budget", timeout))
	defer gcancel()
	judge, err := gradeTrial(gctx, cfg, sb, p.c, u.FinalText, art, &l)
	if judge.Tokens() > 0 {
		jt, jerr := metrics.FromUsage(judge, cfg.JudgePin.Model, cfg.Pricing, 0)
		l.Judge = &jt
		err = errors.Join(err, jerr)
	}
	if _, ok := errors.AsType[infraFault](err); ok {
		return infra(fmt.Errorf("grade: %w", err))
	}
	if err != nil {
		return fail(fmt.Errorf("grade: %w", err))
	}
	return l
}

// infraFault marks a grading-step failure that is the harness's own, not the agent's: an infra error
// even after the agent's output.
type infraFault struct{ error }

// gradeTrial scores the agent's work into l (design.md §5, §7) and returns the judge's usage. An
// impl trial is graded inside the container, which is what first brings in the held-out tests; the
// agent's diff and the verify output land in art as work.patch and verify.log, and a failure's reason
// in l.Error.
func gradeTrial(ctx context.Context, cfg Config, sb Sandbox, c cases.Case, report, art string,
	l *Line,
) (harness.Usage, error) {
	t := &l.Trial
	switch cfg.Role {
	case "impl":
		g, err := grade.GradeImpl(ctx, sb, c)
		if werr := errors.Join(
			os.WriteFile(filepath.Join(art, "work.patch"), []byte(g.Patch), 0o644),
			os.WriteFile(filepath.Join(art, "verify.log"), []byte(g.Log), 0o644)); werr != nil {
			return harness.Usage{}, infraFault{werr}
		}
		if err != nil {
			return harness.Usage{}, err
		}
		t.Passed, t.Work = g.Passed, &g.Work
		t.Score = score(g.Passed)
		l.Error = g.Reason
		return harness.Usage{}, nil
	case "review":
		labels, err := slotLabels(c, cfg.Slot)
		if err != nil {
			return harness.Usage{}, err
		}
		diff, err := os.ReadFile(filepath.Join(c.Dir, "diff.patch"))
		if err != nil {
			return harness.Usage{}, err
		}
		g, err := grade.GradeReview(ctx, report, string(diff), labels, cfg.Judge)
		if err != nil {
			return g.JudgeUsage, err
		}
		t.Review = &g.Metrics
		t.Score = g.Metrics.F1 * 100
		t.Passed = g.Metrics.Recall == 1 // every must-find found
		return g.JudgeUsage, nil
	default:
		var labels grade.VisualLabels
		if err := readJSON(filepath.Join(c.Dir, "labels.json"), &labels); err != nil {
			return harness.Usage{}, err
		}
		g, err := grade.GradeVisual(ctx, report, labels, cfg.Judge)
		if err != nil {
			return g.JudgeUsage, err
		}
		t.Visual = &g.Metrics
		t.Passed = g.Metrics.Correct
		t.Score = score(g.Metrics.Correct)
		return g.JudgeUsage, nil
	}
}

// slotLabels reads review case c's labels.json keeping only the must-finds slot raised; every
// known-false label stays, whichever slot raised it.
func slotLabels(c cases.Case, slot string) (grade.ReviewLabels, error) {
	var labels grade.ReviewLabels
	if err := readJSON(filepath.Join(c.Dir, "labels.json"), &labels); err != nil {
		return labels, err
	}
	labels.MustFind = slices.DeleteFunc(labels.MustFind, func(l grade.Label) bool { return l.Slot != slot })
	return labels, nil
}

func score(pass bool) float64 {
	if pass {
		return 100
	}
	return 0
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// writeJSONLine writes v as one line, map keys sorted so equal records are equal bytes.
func writeJSONLine(w io.Writer, v any) error {
	data, err := json.Marshal(v, json.Deterministic(true))
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// caseRow is one case's figures, computed once and rendered into both the Markdown and the HTML
// report.
type caseRow struct {
	ID                     string
	Valid, K, Passes       int
	PassK                  bool
	Score, StdDev, Cost    float64
	CostPerPass, MeanWallS *float64 // nil when nothing passed, or no trial was valid
	Tokens                 int64
	JudgeCost              float64
	Class                  string // the row's outcome colour, set by paint
}

// summary is what both reports show above the trials.
type summary struct {
	Rows     []caseRow
	Score    float64
	HasScore bool
	How      string // how the role score is computed
	Infra    []Line
	Served   string // the model id(s) its trials were served, per Served
	Class    string // the run's outcome colour, set by paint
}

// paint sets each case row's outcome class and the run's: row-ok when every valid trial passed,
// row-partial when some did, row-fail when none did, and none without a valid trial. base is the
// role's baseline run, nil when the role has none, this run is it, or the two are not Comparable; a
// case scoring above base's same case, or a role score above base's, is row-better instead.
func (s *summary) paint(base *summary) {
	var valid, passes int
	for i, r := range s.Rows {
		valid, passes = valid+r.Valid, passes+r.Passes
		s.Rows[i].Class = outcome(r.Valid, r.Passes)
		if base == nil || r.Valid == 0 {
			continue
		}
		j := slices.IndexFunc(base.Rows, func(b caseRow) bool { return b.ID == r.ID })
		if j >= 0 && base.Rows[j].Valid > 0 && r.Score > base.Rows[j].Score {
			s.Rows[i].Class = "row-better"
		}
	}
	s.Class = outcome(valid, passes)
	if base != nil && s.HasScore && base.HasScore && s.Score > base.Score {
		s.Class = "row-better"
	}
}

func outcome(valid, passes int) string {
	switch {
	case valid == 0:
		return ""
	case passes == valid:
		return "row-ok"
	case passes > 0:
		return "row-partial"
	default:
		return "row-fail"
	}
}

func summarize(h Header, cs []cases.Case, lines []Line) summary {
	byCase := map[string][]metrics.Trial{}
	judgeCost := map[string]float64{}
	s := summary{Served: Served(lines)}
	for _, l := range lines {
		byCase[l.Case] = append(byCase[l.Case], l.Trial)
		if l.Judge != nil {
			judgeCost[l.Case] += l.Judge.CostUSD
		}
		if l.InfraError {
			s.Infra = append(s.Infra, l)
		}
	}
	var scored [][]metrics.Trial
	for _, c := range cs {
		trials := byCase[c.ID]
		a := metrics.CaseAggregate(trials)
		r := caseRow{
			ID: c.ID, Valid: a.Valid, K: h.K, Passes: a.Passes, PassK: a.PassK, Score: a.MeanScore,
			StdDev: a.StdDev, Cost: a.Cost, CostPerPass: a.CostPerPass, Tokens: a.Tokens, JudgeCost: judgeCost[c.ID],
		}
		if a.Valid > 0 {
			r.MeanWallS = new(float64(a.WallMS) / float64(a.Valid) / 1000)
		}
		s.Rows = append(s.Rows, r)
		scored = append(scored, trials)
	}
	s.Score, s.HasScore = metrics.RoleScore(scored)
	s.How = "mean of the case scores"
	if h.Role == "visual" {
		s.How = "balanced accuracy: mean of each labelled verdict's mean case score"
	}
	return s
}

// Load reads runs/<id>.jsonl under runsDir: its header line, then one line per trial.
func Load(runsDir, id string) (Header, []Line, error) {
	f, err := os.Open(filepath.Join(runsDir, id+".jsonl"))
	if err != nil {
		return Header{}, nil, fmt.Errorf("load run %s: %w", id, err)
	}
	defer f.Close()
	var h Header
	var lines []Line
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 16<<20) // a trial line carries a tool-call map; lines stay far below this
	for n := 1; sc.Scan(); n++ {
		var err error
		if n == 1 {
			err = json.Unmarshal(sc.Bytes(), &h)
		} else {
			var l Line
			err = json.Unmarshal(sc.Bytes(), &l)
			lines = append(lines, l)
		}
		if err != nil {
			return Header{}, nil, fmt.Errorf("load run %s: line %d: %w", id, n, err)
		}
	}
	if err := sc.Err(); err != nil {
		return Header{}, nil, fmt.Errorf("load run %s: %w", id, err)
	}
	if h.Role == "" {
		return Header{}, nil, fmt.Errorf("load run %s: no header", id)
	}
	return h, lines, nil
}

// report renders runs/<id>.md: the run's settings, the role score and one row per case, infra-error
// trials excluded from every figure and listed apart.
func report(id string, h Header, s summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Run %s\n\n", id)
	fmt.Fprintf(&b, "- role `%s`, harness `%s`, model `%s`, effort `%s`, k %d, served %s\n",
		h.Role, h.Harness, h.Model, h.Effort, h.K, cmp.Or(s.Served, "–"))
	fmt.Fprintf(&b, "- prompt pin `%s`, case set `%s`, flow-eval `%s`, started %s\n",
		h.PromptPin, h.CaseSetHash, h.FlowEvalSHA, h.Started.Format(time.RFC3339))
	if s.HasScore {
		fmt.Fprintf(&b, "- role score **%.1f** (%s, infra errors excluded)\n", s.Score, s.How)
	}
	b.WriteString("\n| case | valid/k | passes | pass^k | score | stddev | cost/pass | cost | mean wall s | tokens | judge cost |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range s.Rows {
		fmt.Fprintf(&b, "| %s | %d/%d | %d | %s | %.1f | %.1f | %s | $%.4f | %s | %d | $%.4f |\n",
			r.ID, r.Valid, r.K, r.Passes, yesNo(r.PassK), r.Score, r.StdDev, money(r.CostPerPass), r.Cost,
			oneDecimal(r.MeanWallS), r.Tokens, r.JudgeCost)
	}
	if len(s.Infra) > 0 {
		b.WriteString("\n## Infra errors (not scored)\n\n")
		for _, l := range s.Infra {
			fmt.Fprintf(&b, "- %s trial %d: %s\n", l.Case, l.TrialNum, l.Error)
		}
	}
	return b.String()
}

// Served is the distinct model ids lines were served — each line's comma-separated served_model,
// infra-error lines included, since a model that answered was served — sorted and joined with ", ";
// "" when no line names one.
func Served(lines []Line) string {
	var ids []string
	for _, l := range lines {
		for id := range strings.SplitSeq(l.ServedModel, ",") {
			if id != "" && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return strings.Join(ids, ", ")
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// money and oneDecimal render an optional figure, "–" when absent.
func money(v *float64) string {
	if v == nil {
		return "–"
	}
	return fmt.Sprintf("$%.4f", *v)
}

func oneDecimal(v *float64) string {
	if v == nil {
		return "–"
	}
	return fmt.Sprintf("%.1f", *v)
}
