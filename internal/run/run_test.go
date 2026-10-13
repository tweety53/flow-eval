package run

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/tweety53/flow-eval/internal/cases"
	"github.com/tweety53/flow-eval/internal/grade"
	"github.com/tweety53/flow-eval/internal/harness"
	"github.com/tweety53/flow-eval/internal/metrics"
	"github.com/tweety53/flow-eval/internal/sandbox"
)

// hostDocker stands in for the container host with directories on this machine: each trial is a
// fresh directory whose /work and /case are real paths, Exec runs the command for real in /work, and
// CopyIn extracts the tar with the host's tar. Only the docker transport is replaced. Every call is
// logged in order.
type hostDocker struct {
	root      string
	failStart func(n int) bool // fails the n-th Start, 1-based
	atStart   func()           // runs at each Start, before the trial exists
	egressErr error            // EgressDenied's error

	mu     sync.Mutex
	starts int
	log    []string
	repos  []string // each Start's TrialSpec.Repo, in order
}

func (d *hostDocker) record(s string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.log = append(d.log, s)
}

func (d *hostDocker) Start(_ context.Context, spec sandbox.TrialSpec) (Sandbox, error) {
	d.mu.Lock()
	d.starts++
	n := d.starts
	d.repos = append(d.repos, spec.Repo)
	d.mu.Unlock()
	d.record("start " + spec.ID)
	if d.atStart != nil {
		d.atStart()
	}
	if d.failStart != nil && d.failStart(n) {
		return nil, errors.New("docker: daemon unreachable")
	}
	dir := filepath.Join(d.root, spec.ID)
	if err := os.MkdirAll(filepath.Join(dir, "work"), 0o755); err != nil {
		return nil, err
	}
	return &hostTrial{d: d, dir: dir}, nil
}

func (d *hostDocker) Warm(_ context.Context, image, repo, snapshotTar string) error {
	if _, err := os.Stat(snapshotTar); err != nil {
		return err
	}
	d.record("warm " + repo + " " + image)
	return nil
}

type hostTrial struct {
	d   *hostDocker
	dir string
}

func (h *hostTrial) CopyIn(ctx context.Context, tarPath, dir string) error {
	names, err := tarNames(tarPath)
	if err != nil {
		return err
	}
	h.d.record("copyin " + dir + " " + strings.Join(names, ","))
	return exec.CommandContext(ctx, "tar", "-xf", tarPath, "-C", filepath.Join(h.dir, dir)).Run()
}

func (h *hostTrial) Exec(ctx context.Context, cmd ...string) (sandbox.Result, error) {
	h.d.record("exec " + strings.Join(cmd, " "))
	if slices.Equal(cmd, []string{"sh", "-c", "kill -KILL -1"}) {
		return sandbox.Result{}, nil // on the host it would kill every process of the user
	}
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Dir = filepath.Join(h.dir, "work")
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	var stdout, stderr strings.Builder
	c.Stdout, c.Stderr = &stdout, &stderr
	err := c.Run()
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		err = nil
	}
	res := sandbox.Result{Exit: c.ProcessState.ExitCode(), Stdout: stdout.String(), Stderr: stderr.String()}
	if ctx.Err() != nil { // as sandbox.Trial.Exec reports a command its context ended
		return res, fmt.Errorf("exec %s: %w", cmd[0], context.Cause(ctx))
	}
	return res, err
}

func (h *hostTrial) EgressDenied(context.Context) (int, error) { return 2, h.d.egressErr }

func (h *hostTrial) Stop(context.Context) error {
	h.d.record("stop")
	return nil
}

func tarNames(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var names []string
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return names, nil
		}
		if err != nil {
			return nil, err
		}
		names = append(names, hdr.Name)
	}
}

// scriptHarness is an agent that runs script with sh: what it prints is its transcript, and Parse
// reports fixed usage served by model m1 with the transcript as the final message.
type scriptHarness struct{ script string }

func (scriptHarness) Name() string { return "script" }

func (s scriptHarness) Command(promptFile, _, _, _ string) (argv, env []string) {
	return []string{"sh", "-c", s.script, "sh", promptFile}, []string{"SCRIPT_HOME=/tmp/script-home"}
}

func (scriptHarness) Parse(r io.Reader) (harness.Usage, error) {
	out, err := io.ReadAll(r)
	return harness.Usage{
		Input: 1_000_000, Output: 1_000_000, ServedModels: []string{"m1"},
		ToolCalls: map[string]int{"Edit": 1}, FinalText: string(out),
	}, err
}

// sameJudge matches every pair, at a known cost of its own.
type sameJudge struct{}

var _judgeUsage = harness.Usage{Input: 1_000_000, ServedModels: []string{"m1"}}

func (sameJudge) Same(context.Context, string, string) (bool, harness.Usage, error) {
	return true, _judgeUsage, nil
}

func (sameJudge) Plausible(context.Context, string, string) (bool, harness.Usage, error) {
	return true, _judgeUsage, nil
}

var _pricing = metrics.Pricing{"m1": {Input: 1, Output: 10, CacheRead: 0.1, CacheWrite: 1.25, Source: "test"}}

// _judgePin is the judge every test run records; m1 prices its usage.
var _judgePin = grade.Pin{Harness: "claude-code", Model: "m1", Effort: "medium"}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func loadCases(t *testing.T, root, role string) []cases.Case {
	t.Helper()
	cs, invalid, err := cases.LoadAll(root, role)
	if err != nil || len(invalid) > 0 {
		t.Fatalf("load cases: %v %v", err, invalid)
	}
	return cs
}

func visualConfig(t *testing.T, k int) Config {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, filepath.Join(root, "cases", "visual", "v1"), map[string]string{
		"case.json":   `{"id":"v1","role":"visual","source":{"repo":"gymie-frontend"},"notes":"n"}`,
		"labels.json": `{"verdict":"mismatch","defects":[{"summary":"button colour differs"}]}`,
		"mockup.png":  "mockup", "capture.png": "capture",
	})
	return Config{
		Role: "visual", Model: "m1", Effort: "low", Slot: "primary", K: k, Parallel: 2,
		Harness:    scriptHarness{script: `printf -- '- defect: button colour\nverdict: mismatch\n'`},
		Cases:      loadCases(t, filepath.Join(root, "cases"), "visual"),
		PromptsDir: "../../prompts", RunsDir: filepath.Join(root, "runs"),
		RunnerImage: "runner:test", ProxyImage: "proxy:test",
		FlowEvalSHA: "f1a2b3c", Pricing: _pricing, Judge: sameJudge{}, JudgePin: _judgePin,
	}
}

func readRun(t *testing.T, runsDir, id string) (Header, []Line) {
	t.Helper()
	f, err := os.Open(filepath.Join(runsDir, id+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("empty run file")
	}
	var h Header
	if err := json.Unmarshal(sc.Bytes(), &h); err != nil {
		t.Fatal(err)
	}
	var lines []Line
	for sc.Scan() {
		var l Line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, l)
	}
	slices.SortFunc(lines, func(a, b Line) int { return a.TrialNum - b.TrialNum })
	return h, lines
}

func TestRunWritesHeaderAndTrials(t *testing.T) {
	cfg := visualConfig(t, 2)
	d := &hostDocker{root: t.TempDir()}
	id, err := Run(t.Context(), cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(id, "-visual-script-m1-low") {
		t.Errorf("run id %q, want <stamp>-visual-script-m1-low", id)
	}
	h, lines := readRun(t, cfg.RunsDir, id)
	pin, err := os.ReadFile("../../prompts/PIN")
	if err != nil {
		t.Fatal(err)
	}
	setHash, err := cases.SetHash(cfg.Cases)
	if err != nil {
		t.Fatal(err)
	}
	if h.PromptPin != strings.TrimSpace(string(pin)) || h.CaseSetHash != setHash || h.FlowEvalSHA != "f1a2b3c" ||
		h.Role != "visual" || h.Harness != "script" || h.Model != "m1" || h.Effort != "low" || h.K != 2 ||
		h.CaseHashes["v1"] == "" || h.Started.IsZero() {
		t.Errorf("header = %+v", h)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d trial lines, want 2", len(lines))
	}
	for i, l := range lines {
		if l.Case != "v1" || l.TrialNum != i+1 || l.InfraError || !l.Passed || l.Score != 100 ||
			l.Visual == nil || !l.Visual.Correct || l.EgressDenied != 2 {
			t.Errorf("trial line %d = %+v", i, l)
		}
		// 1M input at $1 + 1M output at $10; the judge's 1M input is its own.
		if l.CostUSD != 11 || l.Judge == nil || l.Judge.CostUSD != 1 {
			t.Errorf("trial %d cost %v judge %+v, want 11 and a separate judge cost of 1", i, l.CostUSD, l.Judge)
		}
		art := filepath.Join(cfg.RunsDir, id, "v1", fmt.Sprint(i+1))
		for _, name := range []string{"transcript.jsonl", "output.md", "prompt.md"} {
			if _, err := os.Stat(filepath.Join(art, name)); err != nil {
				t.Errorf("artifact: %v", err)
			}
		}
	}
	md, err := os.ReadFile(filepath.Join(cfg.RunsDir, id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "| v1 | 2/2 | 2 | yes | 100.0 |") {
		t.Errorf("report lacks the v1 row:\n%s", md)
	}
	// Two trials of $11 and 2M tokens each; the judge's $1 per trial is its own column.
	if !regexp.MustCompile(`\| \$22\.0000 \| [0-9.]+ \| 4000000 \| \$2\.0000 \|`).Match(md) {
		t.Errorf("report's v1 row lacks cost 22, tokens 4000000 and judge cost 2:\n%s", md)
	}
}

func TestInfraErrorExcludedFromScore(t *testing.T) {
	cfg := visualConfig(t, 2)
	cfg.Parallel = 1
	d := &hostDocker{root: t.TempDir(), failStart: func(n int) bool { return n == 2 }}
	id, err := Run(t.Context(), cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 2 {
		t.Fatalf("got %d trial lines, want 2", len(lines))
	}
	if bad := lines[1]; !bad.InfraError || !strings.Contains(bad.Error, "daemon unreachable") {
		t.Errorf("failed start recorded as %+v, want an infra error naming the cause", bad)
	}
	if lines[0].InfraError || !lines[0].Passed {
		t.Errorf("good trial = %+v", lines[0])
	}
	md, err := os.ReadFile(filepath.Join(cfg.RunsDir, id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	// One valid trial that passed: the case scores 100, not the 50 a counted failure would give.
	if !strings.Contains(string(md), "| v1 | 1/2 | 1 | yes | 100.0 |") || !strings.Contains(string(md), "daemon unreachable") {
		t.Errorf("report does not exclude the infra error:\n%s", md)
	}
}

func gitRepo(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	writeFiles(t, dir, files)
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "base"},
		{"rev-parse", "HEAD"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		if args[0] == "rev-parse" {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// implConfig is a one-case impl run over a fresh repository src: verify is the case's one verify
// command and script the agent.
func implConfig(t *testing.T, verify, script string) Config {
	t.Helper()
	root := t.TempDir()
	repos := filepath.Join(root, "repos")
	base := gitRepo(t, filepath.Join(repos, "src"), map[string]string{"app.txt": "app\n"})
	caseJSON, err := json.Marshal(map[string]any{
		"id": "i1", "role": "impl", "source": map[string]string{"repo": "src", "change": "c"},
		"snapshot": base, "notes": "n", "verify": []string{verify}, "files": []string{"feature.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, filepath.Join(root, "cases", "impl", "i1"), map[string]string{
		"case.json": string(caseJSON), "task.md": "t", "context.md": "c",
		"heldout.patch":  "diff --git a/held.txt b/held.txt\nnew file mode 100644\n--- /dev/null\n+++ b/held.txt\n@@ -0,0 +1 @@\n+x\n",
		"reference.stat": " feature.txt | 1 +\n",
	})
	return Config{
		Role: "impl", Model: "m1", Effort: "low", Slot: "primary", K: 1, Parallel: 1,
		Harness: scriptHarness{script: script}, Cases: loadCases(t, filepath.Join(root, "cases"), "impl"),
		ReposDir: repos, PromptsDir: "../../prompts", RunsDir: filepath.Join(root, "runs"),
		RunnerImage: "r", ProxyImage: "p", FlowEvalSHA: "f", Pricing: _pricing, Judge: sameJudge{}, JudgePin: _judgePin,
	}
}

func TestReviewGradedOnItsSlotsMustFinds(t *testing.T) {
	root := t.TempDir()
	repos := filepath.Join(root, "repos")
	head := gitRepo(t, filepath.Join(repos, "src"), map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	for id, labels := range map[string]string{
		// primary raised F1; principles raised F2, which a primary trial is not graded on, and F3, a
		// known-false label every slot's trial is graded on.
		"mixed": `{"mustFind":[{"id":"F1","file":"a.txt","line":1,"summary":"a","severity":"Important","slot":"primary"},` +
			`{"id":"F2","file":"b.txt","line":1,"summary":"b","severity":"Important","slot":"principles"}],` +
			`"knownFalse":[{"id":"F3","file":"a.txt","line":1,"summary":"c","slot":"principles"}]}`,
		"others": `{"mustFind":[{"id":"F1","file":"b.txt","line":1,"summary":"b","slot":"principles"}],"knownFalse":[]}`,
		"more":   `{"mustFind":[{"id":"F1","file":"a.txt","line":1,"summary":"a","slot":"principles"}],"knownFalse":[]}`,
	} {
		writeFiles(t, filepath.Join(root, "cases", "review", id), map[string]string{
			"case.json": `{"id":"` + id + `","role":"review","source":{"repo":"src","change":"c"},"snapshot":"` + head +
				`","notes":"n"}`,
			"diff.patch": "d\n", "labels.json": labels,
		})
	}
	all := loadCases(t, filepath.Join(root, "cases"), "review")
	config := func(cs []cases.Case) Config {
		return Config{
			Role: "review", Model: "m1", Effort: "low", Slot: "primary", K: 1, Parallel: 1,
			Harness: scriptHarness{script: `printf -- '#### Important\n- a.txt:1 the defect\n'`}, Cases: cs,
			ReposDir: repos, PromptsDir: "../../prompts", RunsDir: filepath.Join(root, "runs"),
			RunnerImage: "r", ProxyImage: "p", FlowEvalSHA: "f", Pricing: _pricing, Judge: sameJudge{}, JudgePin: _judgePin,
		}
	}

	t.Run("a primary trial is graded on primary's must-finds only", func(t *testing.T) {
		mixed := all[slices.IndexFunc(all, func(c cases.Case) bool { return c.ID == "mixed" })]
		cfg := config([]cases.Case{mixed})
		id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		_, lines := readRun(t, cfg.RunsDir, id)
		if len(lines) != 1 || lines[0].Review == nil || lines[0].Review.Recall != 1 || !lines[0].Passed {
			t.Fatalf("trial = %+v, want recall 1: F2 is principles', not primary's", lines)
		}
		if got := lines[0].Review.KnownFalseHits; got != 1 {
			t.Errorf("known_false_hits = %d, want 1: principles' known-false F3 applies to a primary trial", got)
		}
	})
	t.Run("a slot with no prompt is named before the must-find refusal", func(t *testing.T) {
		cfg := config(all)
		cfg.Slot = "bogus"
		cfg.RunsDir = filepath.Join(t.TempDir(), "runs")
		_, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
		if err == nil || !strings.Contains(err.Error(), "review-bogus.md") {
			t.Errorf("Run = %v, want an error naming the missing prompt review-bogus.md", err)
		}
	})
	t.Run("a mis-cased slot is refused for its missing prompt, not matched case-insensitively", func(t *testing.T) {
		cfg := config(all)
		cfg.Slot = "Primary"
		cfg.RunsDir = filepath.Join(t.TempDir(), "runs")
		_, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
		if err == nil || !strings.Contains(err.Error(), "review-Primary.md") {
			t.Errorf("Run = %v, want an error naming the missing prompt review-Primary.md", err)
		}
	})
	t.Run("every case with no must-find for the slot is refused before any trial", func(t *testing.T) {
		cfg := config(all)
		cfg.RunsDir = filepath.Join(t.TempDir(), "runs")
		d := &hostDocker{root: t.TempDir()}
		_, err := Run(t.Context(), cfg, d)
		if err == nil || !strings.Contains(err.Error(), "others") || !strings.Contains(err.Error(), "more") ||
			!strings.Contains(err.Error(), "primary") {
			t.Errorf("Run = %v, want a refusal naming cases others and more, and slot primary", err)
		}
		if len(d.log) > 0 {
			t.Errorf("docker calls %v, want none before the refusal", d.log)
		}
		if _, err := os.Stat(cfg.RunsDir); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("runs directory: %v, want none written", err)
		}
	})
}

func TestHeldoutAppliedAfterAgentExit(t *testing.T) {
	root := t.TempDir()
	repos := filepath.Join(root, "repos")
	base := gitRepo(t, filepath.Join(repos, "src"), map[string]string{"app.txt": "app\n"})
	heldout := "diff --git a/held.txt b/held.txt\nnew file mode 100644\n--- /dev/null\n+++ b/held.txt\n@@ -0,0 +1 @@\n+HELDOUT_MARK\n"
	writeFiles(t, filepath.Join(root, "cases", "impl", "i1"), map[string]string{
		"case.json": `{"id":"i1","role":"impl","source":{"repo":"src","change":"c"},"snapshot":"` + base +
			`","notes":"n","verify":["test -f held.txt && test -f feature.txt"],"files":["feature.txt"]}`,
		"task.md": "add feature.txt", "context.md": "ctx", "heldout.patch": heldout,
		"reference.stat": " feature.txt | 1 +\n 1 file changed, 1 insertion(+)\n",
	})
	cfg := Config{
		Role: "impl", Model: "m1", Effort: "low", Slot: "primary", K: 1, Parallel: 1,
		// The agent fails the trial if the held-out patch is anywhere it can see.
		Harness: scriptHarness{script: `test ! -e .git/flow-eval-heldout.patch && test ! -e held.txt && ` +
			`test -f ../case/prompt.md && ! grep -rqs HELDOUT_MARK ../case && echo feature > feature.txt && echo done`},
		Cases:      loadCases(t, filepath.Join(root, "cases"), "impl"),
		ReposDir:   repos,
		PromptsDir: "../../prompts", RunsDir: filepath.Join(root, "runs"),
		RunnerImage: "runner:test", ProxyImage: "proxy:test",
		FlowEvalSHA: "f1a2b3c", Pricing: _pricing, Judge: sameJudge{}, JudgePin: _judgePin,
	}
	d := &hostDocker{root: t.TempDir()}
	id, err := Run(t.Context(), cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 1 || !lines[0].Passed || lines[0].Work == nil || lines[0].Work.DiffLines != 1 {
		t.Fatalf("trial = %+v, want a pass graded on the agent's one-line feature", lines)
	}
	at := func(prefix string) int {
		return slices.IndexFunc(d.log, func(s string) bool { return strings.HasPrefix(s, prefix) })
	}
	warm, start := at("warm src runner:test"), at("start ")
	agent, kill := at("exec env SCRIPT_HOME=/tmp/script-home sh -c"), at("exec sh -c kill -KILL -1")
	held := at("copyin /work/.git ")
	if warm < 0 || start < warm || agent < start || kill < agent || held < kill {
		t.Errorf("want warm < start < agent < kill < held-out copy, got %d %d %d %d %d:\n%s",
			warm, start, agent, kill, held, strings.Join(d.log, "\n"))
	}
	for _, s := range d.log[:held] {
		if strings.HasPrefix(s, "copyin ") && (strings.Contains(s, "heldout") || strings.Contains(s, "reference.stat")) {
			t.Errorf("held-out material reached the container before the agent exited: %s", s)
		}
	}
}

// A visual trial never builds, so its sandbox gets no repository and so no Gradle volume; an impl
// trial gets its case's repository.
func TestOnlyBuildingTrialsGetTheGradleVolume(t *testing.T) {
	d := &hostDocker{root: t.TempDir()}
	if _, err := Run(t.Context(), visualConfig(t, 1), d); err != nil {
		t.Fatal(err)
	}
	cfg := implConfig(t, "true", "echo feature > feature.txt && echo done")
	if _, err := Run(t.Context(), cfg, d); err != nil {
		t.Fatal(err)
	}
	if want := []string{"", "src"}; !slices.Equal(d.repos, want) {
		t.Errorf("trial repos = %q, want %q", d.repos, want)
	}
}

// A verify command that never returns is cut off at the grading deadline: the trial is a scored fail
// naming the deadline, and the run finishes.
func TestGradingHasADeadline(t *testing.T) {
	old := Timeouts["impl"]
	Timeouts["impl"] = 2 * time.Second
	t.Cleanup(func() { Timeouts["impl"] = old })
	cfg := implConfig(t, "exec sleep 60", "echo feature > feature.txt && echo done")
	began := time.Now()
	id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > 30*time.Second {
		t.Errorf("run took %s; grading was not cut off", took)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 1 || lines[0].InfraError || lines[0].Passed || !strings.Contains(lines[0].Error, "grading exceeded its 2s budget") {
		t.Fatalf("hung verify recorded as %+v, want a scored fail naming the grading deadline", lines)
	}
}

// A run cancelled while a trial is grading records that trial as an infra error, not a scored fail:
// the candidate did not cause it.
func TestRunCancelledDuringGradingIsInfra(t *testing.T) {
	cfg := implConfig(t, "exec sleep 60", "echo feature > feature.txt && echo done")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	d := &hostDocker{root: t.TempDir()}
	go func() { // cancel once the verify command is running
		for {
			d.mu.Lock()
			grading := slices.Contains(d.log, "exec sh -c exec sleep 60")
			d.mu.Unlock()
			if grading {
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	id, err := Run(ctx, cfg, d)
	if err == nil {
		t.Fatal("a cancelled run returned no error")
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 1 || !lines[0].InfraError {
		t.Fatalf("trial of a cancelled run recorded as %+v, want an infra error", lines)
	}
	for _, name := range []string{id + ".md", id + ".html", "index.html"} {
		if _, err := os.Stat(filepath.Join(cfg.RunsDir, name)); err != nil {
			t.Errorf("a cancelled run's report: %v", err)
		}
	}
}

// unpricedJudge matches every pair, served by a model pricing.json lacks.
type unpricedJudge struct{}

func (unpricedJudge) Same(context.Context, string, string) (bool, harness.Usage, error) {
	return true, harness.Usage{Input: 1, ServedModels: []string{"unpriced"}}, nil
}

func (unpricedJudge) Plausible(context.Context, string, string) (bool, harness.Usage, error) {
	return true, harness.Usage{Input: 1, ServedModels: []string{"unpriced"}}, nil
}

// A judge whose usage cannot be priced fails the trial's grading: a scored fail, never infra.
func TestJudgePricingFailureIsScored(t *testing.T) {
	cfg := visualConfig(t, 1)
	cfg.Judge, cfg.JudgePin.Model = unpricedJudge{}, "unpriced"
	id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 1 || lines[0].InfraError || lines[0].Passed || !strings.Contains(lines[0].Error, "grade: price model unpriced") {
		t.Fatalf("trial recorded as %+v, want a scored fail naming the unpriced judge model", lines)
	}
}

// A failed impl trial says why in its line, and keeps the agent's diff and the verify output in its
// trial directory.
func TestImplFailureIsRecorded(t *testing.T) {
	cfg := implConfig(t, "echo VERIFY_OUT_MARK; exit 1", "echo AGENT_DIFF_MARK > feature.txt && echo done")
	id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 1 || lines[0].InfraError || lines[0].Passed || lines[0].Error != "verify: echo VERIFY_OUT_MARK; exit 1: exit 1" {
		t.Fatalf("failed trial recorded as %+v, want its verify reason", lines)
	}
	art := filepath.Join(cfg.RunsDir, id, "i1", "1")
	for name, mark := range map[string]string{"work.patch": "+AGENT_DIFF_MARK", "verify.log": "VERIFY_OUT_MARK\n[exit 1]"} {
		data, err := os.ReadFile(filepath.Join(art, name))
		if err != nil || !strings.Contains(string(data), mark) {
			t.Errorf("%s = %q, %v; want it to hold %q", name, data, err, mark)
		}
	}
}

// A grading failure the agent caused — here git's index lock left in its work tree — is a scored
// fail, never an infra error the aggregates would skip.
func TestAgentCausedGradeFailureIsScored(t *testing.T) {
	cfg := implConfig(t, "true", "echo feature > feature.txt && touch .git/index.lock && echo done")
	id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 1 || lines[0].InfraError || lines[0].Passed || !strings.HasPrefix(lines[0].Error, "grade: ") {
		t.Fatalf("trial recorded as %+v, want a scored fail with a grade: reason", lines)
	}
	md, err := os.ReadFile(filepath.Join(cfg.RunsDir, id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "| i1 | 1/1 | 0 | no | 0.0 |") {
		t.Errorf("report does not score the failed trial:\n%s", md)
	}
}

// The run report's visual role score is balanced accuracy: a verifier that answers mismatch to one
// match case and two mismatch cases is right on one class only, and scores 50, not the 66.7 of
// plain accuracy.
func TestVisualRoleScoreIsBalancedAccuracy(t *testing.T) {
	cfg := visualConfig(t, 1)
	dir := filepath.Dir(cfg.Cases[0].Dir)
	for id, labels := range map[string]string{
		"v2": `{"verdict":"match","defects":[]}`,
		"v3": `{"verdict":"mismatch","defects":[{"summary":"button colour differs"}]}`,
	} {
		writeFiles(t, filepath.Join(dir, id), map[string]string{
			"case.json":   `{"id":"` + id + `","role":"visual","source":{"repo":"gymie-frontend"},"notes":"n"}`,
			"labels.json": labels,
			"mockup.png":  "mockup", "capture.png": "capture",
		})
	}
	cfg.Cases = loadCases(t, filepath.Dir(dir), "visual")
	id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(cfg.RunsDir, id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "- role score **50.0** (balanced accuracy") {
		t.Errorf("report lacks a balanced role score of 50:\n%s", md)
	}
}

// noResultHarness parses like claude-code: a transcript cut off before its result event is an error.
type noResultHarness struct{ scriptHarness }

func (noResultHarness) Parse(io.Reader) (harness.Usage, error) {
	return harness.Usage{}, errors.New("transcript has no result event")
}

func TestTimedOutTrialIsScoredNotInfra(t *testing.T) {
	old := Timeouts["visual"]
	Timeouts["visual"] = 300 * time.Millisecond
	t.Cleanup(func() { Timeouts["visual"] = old })
	cfg := visualConfig(t, 1)
	cfg.Harness = noResultHarness{scriptHarness{script: `printf '{"type":"system"}\n'; sleep 5`}}
	id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 1 || lines[0].InfraError || !lines[0].TimedOut || lines[0].Passed {
		t.Fatalf("timed-out trial recorded as %+v, want a scored, failed, timed_out trial", lines)
	}
}

// Two runs stamped in the same second never share a sandbox name, and a second run with the same id
// is refused before any container starts rather than overwriting the first run's record.
func TestRunIDsNeverShareSandboxesOrRecords(t *testing.T) {
	stamp := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	now = func() time.Time { return stamp }
	t.Cleanup(func() { now = time.Now })
	d := &hostDocker{root: t.TempDir()}
	cfg := visualConfig(t, 1)
	first, err := Run(t.Context(), cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	other := cfg
	other.Model = "m2"
	if _, err := Run(t.Context(), other, d); err != nil {
		t.Fatal(err)
	}
	starts := slices.DeleteFunc(slices.Clone(d.log), func(s string) bool { return !strings.HasPrefix(s, "start ") })
	if len(starts) != 2 || starts[0] == starts[1] {
		t.Errorf("sandbox starts %v, want two distinct names", starts)
	}
	before, err := os.ReadFile(filepath.Join(cfg.RunsDir, first+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(t.Context(), cfg, d); err == nil {
		t.Error("a second run with the same id was not refused")
	}
	after, err := os.ReadFile(filepath.Join(cfg.RunsDir, first+".jsonl"))
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("the first run's record changed: %v", err)
	}
	if n := len(slices.DeleteFunc(slices.Clone(d.log), func(s string) bool { return !strings.HasPrefix(s, "start ") })); n != 2 {
		t.Errorf("%d sandbox starts, want none for the refused run", n)
	}
}

// A failure of the harness's own after the agent's output — pricing the trial's model, counting its
// egress denials, writing an artifact — is an infra error, not a fail charged to the candidate.
func TestHarnessSideFailureAfterOutputIsInfra(t *testing.T) {
	const script = "echo feature > feature.txt && echo done"
	// blockArtifact makes the trial directory's name a directory, so writing that artifact fails.
	blockArtifact := func(cfg Config, name string) func() {
		return func() {
			arts, _ := filepath.Glob(filepath.Join(cfg.RunsDir, "*", "i1", "1"))
			for _, art := range arts {
				if err := os.Mkdir(filepath.Join(art, name), 0o755); err != nil {
					t.Error(err)
				}
			}
		}
	}
	t.Run("the trial's model is not priced", func(t *testing.T) {
		cfg := implConfig(t, "true", script)
		cfg.Pricing = metrics.Pricing{"other": _pricing["m1"]}
		assertInfra(t, cfg, &hostDocker{root: t.TempDir()}, "price model m1")
	})
	t.Run("egress denials cannot be counted", func(t *testing.T) {
		cfg := implConfig(t, "true", script)
		d := &hostDocker{root: t.TempDir(), egressErr: errors.New("proxy log unreadable")}
		assertInfra(t, cfg, d, "count egress denials: proxy log unreadable")
	})
	t.Run("output.md cannot be written", func(t *testing.T) {
		cfg := implConfig(t, "true", script)
		assertInfra(t, cfg, &hostDocker{root: t.TempDir(), atStart: blockArtifact(cfg, "output.md")}, "output.md")
	})
	t.Run("work.patch cannot be written", func(t *testing.T) {
		cfg := implConfig(t, "true", script)
		assertInfra(t, cfg, &hostDocker{root: t.TempDir(), atStart: blockArtifact(cfg, "work.patch")}, "grade: open")
	})
}

// outageHarness is an agent whose run the API ended before its answer.
type outageHarness struct{ scriptHarness }

func (h outageHarness) Parse(r io.Reader) (harness.Usage, error) {
	u, err := h.scriptHarness.Parse(r)
	u.FinalText, u.Outage = "", "API error 429: quota exhausted"
	return u, err
}

// A run the API ended before the agent's answer is an infra error, never a fail charged to it.
func TestAPIOutageIsInfra(t *testing.T) {
	cfg := visualConfig(t, 1)
	cfg.Harness = outageHarness{scriptHarness{script: "echo partial"}}
	assertInfra(t, cfg, &hostDocker{root: t.TempDir()}, "agent API outage: API error 429: quota exhausted")
}

func assertInfra(t *testing.T, cfg Config, d *hostDocker, reason string) {
	t.Helper()
	id, err := Run(t.Context(), cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	if len(lines) != 1 || !lines[0].InfraError || !strings.Contains(lines[0].Error, reason) {
		t.Fatalf("trial recorded as %+v, want an infra error naming %q", lines, reason)
	}
}

func TestRunWritesHTMLPage(t *testing.T) {
	runPage := func(t *testing.T, cfg Config) string {
		t.Helper()
		id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(cfg.RunsDir, id+".html"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	t.Run("visual: header, case row, score, metrics, escaped output", func(t *testing.T) {
		cfg := visualConfig(t, 2)
		cfg.Harness = scriptHarness{script: `printf -- '<script>alert(1)</script>\n- defect: button colour\nverdict: mismatch\n'`}
		html := runPage(t, cfg)
		for _, want := range []string{
			"<!doctype html>", ">visual<", ">m1<", ">f1a2b3c<", // header
			"<td>v1</td><td>2/2</td><td>2</td><td>yes</td><td>100.0</td>", // case row
			"Role score", "balanced accuracy", // score and how it is computed
			"cost_usd", "egress_denied", "&#34;correct&#34;:true", // every metric, nested ones as JSON
			"&lt;script&gt;alert(1)&lt;/script&gt;",          // the agent's output, escaped
			"v1/1/transcript.jsonl", "v1/2/transcript.jsonl", // each trial
		} {
			if !strings.Contains(html, want) {
				t.Errorf("page lacks %q", want)
			}
		}
		if strings.Contains(html, "<script>") {
			t.Error("page carries an unescaped <script>")
		}
	})
	t.Run("trials in case order, then trial order, whatever order they finished in", func(t *testing.T) {
		s := summary{Rows: []caseRow{{ID: "b"}, {ID: "a"}}}
		lines := []Line{{Case: "a", TrialNum: 1}, {Case: "b", TrialNum: 2}, {Case: "b", TrialNum: 1}}
		html, err := page(t.TempDir(), "r", Header{}, s, lines)
		if err != nil {
			t.Fatal(err)
		}
		var at []int
		for _, p := range []string{"r/b/1/transcript.jsonl", "r/b/2/transcript.jsonl", "r/a/1/transcript.jsonl"} {
			at = append(at, strings.Index(string(html), p))
		}
		if !slices.IsSorted(at) || at[0] < 0 {
			t.Errorf("trial positions %v, want b/1, b/2, a/1 in that order", at)
		}
	})
	t.Run("figures as recorded, a long artifact cut", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, filepath.Join(dir, "r", "c", "1"), map[string]string{"verify.log": strings.Repeat("x", 256<<10-1) + "é" + strings.Repeat("x", 50<<10) + "TAIL"})
		lines := []Line{{Case: "c", TrialNum: 1, Trial: metrics.Trial{
			CacheRead: 1_500_000, WallMS: 1_234_567, ServedModel: "m9",
			ToolCalls: map[string]int{"F": 6, "E": 5, "D": 4, "C": 3, "B": 2, "A": 1},
		}}}
		data, err := page(dir, "r", Header{}, summary{Rows: []caseRow{{ID: "c"}}}, lines)
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.Valid(data) {
			t.Error("page is not valid UTF-8: a long artifact was cut inside a character")
		}
		html := string(data)
		for _, want := range []string{"<dd>1500000</dd>", "<dd>1234567</dd>", "<dd>m9</dd>",
			"served_matches_requested</dt><dd>–</dd>",
			"{&#34;A&#34;:1,&#34;B&#34;:2,&#34;C&#34;:3,&#34;D&#34;:4,&#34;E&#34;:5,&#34;F&#34;:6}",
			"cut at 256 KiB; the whole file is r/c/1/verify.log"} {
			if !strings.Contains(html, want) {
				t.Errorf("page lacks %q", want)
			}
		}
		for _, bad := range []string{"e+06", "&lt;nil&gt;", "TAIL"} {
			if strings.Contains(html, bad) {
				t.Errorf("page carries %q", bad)
			}
		}
	})
	t.Run("impl: error, verify log and diff", func(t *testing.T) {
		cfg := implConfig(t, "echo VERIFY_OUT_MARK; exit 1", "echo AGENT_DIFF_MARK > feature.txt && echo done")
		html := runPage(t, cfg)
		for _, want := range []string{"verify: echo VERIFY_OUT_MARK; exit 1: exit 1", "VERIFY_OUT_MARK\n[exit 1]", "work.patch</summary>", "AGENT_DIFF_MARK"} {
			if !strings.Contains(html, want) {
				t.Errorf("page lacks %q", want)
			}
		}
	})
}

func TestCaseRowMeanWallIsPerValidTrial(t *testing.T) {
	lines := []Line{
		{Case: "c", TrialNum: 1, Trial: metrics.Trial{WallMS: 3000}},
		{Case: "c", TrialNum: 2, Trial: metrics.Trial{WallMS: 1000}},
		{Case: "c", TrialNum: 3, Trial: metrics.Trial{WallMS: 9000, InfraError: true}},
	}
	s := summarize(Header{K: 3}, []cases.Case{{ID: "c"}}, lines)
	if got := oneDecimal(s.Rows[0].MeanWallS); got != "2.0" {
		t.Errorf("mean wall = %s s, want 2.0 (infra trial excluded)", got)
	}
	if got := oneDecimal(summarize(Header{K: 1}, []cases.Case{{ID: "none"}}, nil).Rows[0].MeanWallS); got != "–" {
		t.Errorf("mean wall with no valid trial = %q, want –", got)
	}
}

func TestRunWritesIndexOfEveryRun(t *testing.T) {
	at := func(hour int) func() time.Time {
		return func() time.Time { return time.Date(2026, 10, 10, hour, 0, 0, 0, time.UTC) }
	}
	t.Cleanup(func() { now = time.Now })
	visual := visualConfig(t, 2)
	impl := implConfig(t, "true", "echo feature > feature.txt && echo done")
	impl.RunsDir = visual.RunsDir
	run := func(cfg Config, hour int) string {
		t.Helper()
		now = at(hour)
		id, err := Run(t.Context(), cfg, &hostDocker{root: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	older := run(visual, 9)
	newer := run(visual, 11)
	writeFiles(t, visual.RunsDir, map[string]string{"broken.jsonl": "not json\n"})
	// A run recorded before the HTML page existed is listed without a link.
	if err := os.Rename(filepath.Join(visual.RunsDir, older+".jsonl"), filepath.Join(visual.RunsDir, "old.jsonl")); err != nil {
		t.Fatal(err)
	}
	implID := run(impl, 10)

	data, err := os.ReadFile(filepath.Join(visual.RunsDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	index := string(data)
	var at2 []int
	for _, want := range []string{
		`<h2 id="impl">impl</h2>`, `href="./` + implID + `.html"`,
		`<h2 id="visual">visual</h2>`, `href="./` + newer + `.html"`, `old <span class="muted">(no page)</span>`,
	} {
		i := strings.Index(index, want)
		if i < 0 {
			t.Fatalf("index lacks %q", want)
		}
		at2 = append(at2, i)
	}
	if !slices.IsSorted(at2) {
		t.Errorf("index order %v, want impl, then visual newest first", at2)
	}
	if !strings.Contains(index, "Unreadable run files") || !strings.Contains(index, "load run broken: line 1") {
		t.Error("index does not name the unreadable run file")
	}
	// k 2, one case, score 100, valid 2 of 2, 2 passes, no infra error, $22 cost, $2 judge cost.
	if !strings.Contains(index, "<td>2</td><td>1</td><td>100.0</td><td>2/2</td><td>2</td><td>0</td><td>$22.0000</td><td>$2.0000</td>") {
		t.Error("index lacks the visual runs' figures")
	}

	page, err := os.ReadFile(filepath.Join(visual.RunsDir, newer+".html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="index.html"`, `href="#case-v1"`, `id="case-v1"`, `href="#trial-v1-1"`, `id="trial-v1-1"`} {
		if !strings.Contains(string(page), want) {
			t.Errorf("run page lacks %q", want)
		}
	}
}

func TestIndexListsEveryRunFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		// A role the index does not know, an id with a colon, and a case no trial line reached.
		"r-qwen:7b.jsonl": `{"role":"judge","k":1,"case_hashes":{"a":"x","b":"y"},"started":"2026-10-10T09:00:00Z"}` + "\n" +
			`{"case":"a","trial":1,"score":100,"passed":true}` + "\n",
		"r-qwen:7b.html": "page",
		// A header naming no cases: the trial lines name them.
		"bare.jsonl": `{"role":"review","k":2,"started":"2026-10-10T08:00:00Z"}` + "\n" +
			`{"case":"c","trial":1,"score":0}` + "\n" + `{"case":"c","trial":2,"score":0}` + "\n",
	})
	old, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if _, err := old.WriteString("previous index"); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(Repaint(dir, t.TempDir())); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	index := string(data)
	for _, want := range []string{`<h2 id="judge">judge</h2>`, `href="./r-qwen:7b.html"`, "<td>1</td><td>2</td><td>100.0</td><td>1/1</td>", "<td>2</td><td>1</td><td>0.0</td><td>2/2</td>"} {
		if !strings.Contains(index, want) {
			t.Errorf("index lacks %q", want)
		}
	}
	// The index is replaced by a rename, never rewritten in place: a reader holding the previous
	// file still reads it whole.
	if _, err := old.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	held, err := io.ReadAll(old)
	if err != nil || string(held) != "previous index" {
		t.Errorf("a reader of the previous index got %q, %v; want it untouched", held, err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, ".index.html-*")); len(leftovers) > 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func TestIndexReadsARunsPathHoldingPatternCharacters(t *testing.T) {
	for _, name := range []string{"runs[old]", "runs["} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), name)
			writeFiles(t, dir, map[string]string{"r1.jsonl": `{"role":"review","k":1,"started":"2026-10-10T08:00:00Z"}` + "\n"})
			if err := errors.Join(Repaint(dir, t.TempDir())); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "index.html"))
			if err != nil || !strings.Contains(string(data), "r1") {
				t.Errorf("index of %s = %v, does not list r1", name, err)
			}
		})
	}
}

func TestIndexWriteFailureLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	// index.html as a non-empty directory: the rename over it fails.
	writeFiles(t, dir, map[string]string{"index.html/keep": "x"})
	if _, err := Repaint(dir, t.TempDir()); err == nil {
		t.Fatal("writeIndex over a directory succeeded, want an error")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, ".index.html-*")); len(leftovers) > 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func TestRowsPaintedByOutcome(t *testing.T) {
	dir, baselines := t.TempDir(), t.TempDir()
	runFile := func(role, started string, trials ...string) string {
		return fmt.Sprintf(`{"role":%q,"k":2,"case_hashes":{"a":"x","b":"x","c":"x","d":"x"},"started":%q}`, role, started) +
			"\n" + strings.Join(trials, "\n") + "\n"
	}
	pass := func(c string, n int) string {
		return fmt.Sprintf(`{"case":%q,"trial":%d,"score":100,"passed":true}`, c, n)
	}
	fail := func(c string, n int) string { return fmt.Sprintf(`{"case":%q,"trial":%d,"score":0}`, c, n) }
	infra := func(c string, n int) string { return fmt.Sprintf(`{"case":%q,"trial":%d,"infra_error":true}`, c, n) }
	writeFiles(t, dir, map[string]string{
		// The review baseline: a all pass, b partly, c and d none.
		"base.jsonl": runFile("review", "2026-10-10T08:00:00Z",
			pass("a", 1), pass("a", 2), pass("b", 1), fail("b", 2), fail("c", 1), fail("c", 2), fail("d", 1), fail("d", 2)),
		// One case score above the baseline on b, level on a, c and d.
		"cand.jsonl": runFile("review", "2026-10-10T09:00:00Z",
			pass("a", 1), pass("a", 2), pass("b", 1), pass("b", 2), fail("c", 1), fail("c", 2), fail("d", 1), fail("d", 2)),
		// impl has no baseline: no row is blue. d has only infra errors.
		"solo.jsonl": runFile("impl", "2026-10-10T09:00:00Z",
			pass("a", 1), pass("a", 2), pass("b", 1), pass("b", 2), infra("d", 1), infra("d", 2)),
		"infra.jsonl": runFile("impl", "2026-10-10T08:00:00Z", infra("a", 1)),
		// visual's baseline names a run that no longer exists: no row is blue, the index still writes.
		"vis.jsonl": runFile("visual", "2026-10-10T09:00:00Z", pass("a", 1), pass("a", 2)),
		// Pages from before the colours, re-rendered from their JSONL; an unreadable run keeps its page.
		"base.html": "old page", "cand.html": "old page", "solo.html": "old page", "vis.html": "old page",
		"broken.jsonl": "not json\n", "broken.html": "kept",
	})
	writeFiles(t, baselines, map[string]string{
		"review.json": `{"runId":"base"}`,
		"visual.json": `{"runId":"gone"}`,
	})
	if err := errors.Join(Repaint(dir, baselines)); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for file, wants := range map[string][]string{
		"base.html": {`<tr id="case-a" class="row-ok">`, `<tr id="case-b" class="row-partial">`,
			`<tr id="case-c" class="row-fail">`, `<tr id="case-d" class="row-fail">`, `class="legend"`},
		"cand.html": {`<tr id="case-a" class="row-ok">`, `<tr id="case-b" class="row-better">`,
			`<tr id="case-c" class="row-fail">`, `<tr id="case-d" class="row-fail">`},
		"solo.html": {`<tr id="case-a" class="row-ok">`, `<tr id="case-b" class="row-ok">`, `<tr id="case-d">`},
		"vis.html":  {`<tr id="case-a" class="row-ok">`},
		"index.html": {`<tr class="row-partial"><td><a href="./base.html">`, `<tr class="row-better"><td><a href="./cand.html">`,
			`<tr class="row-ok"><td><a href="./solo.html">`, `<tr><td>infra <span class="muted">(no page)</span>`,
			`<tr class="row-ok"><td><a href="./vis.html">`, `class="legend"`},
		"broken.html": {"kept"},
	} {
		got := read(file)
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("%s lacks %q", file, want)
			}
		}
	}
	for _, file := range []string{"base.html", "solo.html", "vis.html"} {
		if regexp.MustCompile(`<tr[^>]*row-better`).MatchString(read(file)) {
			t.Errorf("%s carries a blue row", file)
		}
	}
}

// A row is blue only over a baseline compare would compare it with: the same role, slot, case set
// and k, under any prompt pin, and k valid trials of every case on both sides.
func TestNoBlueOverABaselineCompareRefuses(t *testing.T) {
	const base = `{"role":"review","slot":"s","k":1,"prompt_pin":"p","case_set_hash":"ab","case_hashes":{"a":"x","b":"x"},"started":"2026-10-10T08:00:00Z"}
{"case":"a","trial":1,"score":100,"passed":true}
{"case":"b","trial":1,"score":0}
`
	const both = `{"case":"a","trial":1,"score":100,"passed":true}
{"case":"b","trial":1,"score":100,"passed":true}
`
	for _, tc := range []struct {
		name, header, trials string
		blue                 bool
	}{
		{"comparable", `"slot":"s","k":1,"prompt_pin":"p","case_set_hash":"ab","case_hashes":{"a":"x","b":"x"}`, both, true},
		{"a subset of the cases", `"slot":"s","k":1,"prompt_pin":"p","case_set_hash":"a","case_hashes":{"a":"x"}`,
			`{"case":"a","trial":1,"score":100,"passed":true}` + "\n", false},
		{"a changed case", `"slot":"s","k":1,"prompt_pin":"p","case_set_hash":"ab2","case_hashes":{"a":"x","b":"y"}`, both, false},
		{"another slot", `"slot":"t","k":1,"prompt_pin":"p","case_set_hash":"ab","case_hashes":{"a":"x","b":"x"}`, both, false},
		{"another prompt pin", `"slot":"s","k":1,"prompt_pin":"q","case_set_hash":"ab","case_hashes":{"a":"x","b":"x"}`, both, true},
		{"another k", `"slot":"s","k":2,"prompt_pin":"p","case_set_hash":"ab","case_hashes":{"a":"x","b":"x"}`,
			both + `{"case":"a","trial":2,"score":100,"passed":true}` + "\n" + `{"case":"b","trial":2,"score":100,"passed":true}` + "\n", false},
		{"a case short of k valid trials", `"slot":"s","k":1,"prompt_pin":"p","case_set_hash":"ab","case_hashes":{"a":"x","b":"x"}`,
			`{"case":"a","trial":1,"score":100,"passed":true}` + "\n" + `{"case":"b","trial":1,"infra_error":true}` + "\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, baselines := t.TempDir(), t.TempDir()
			writeFiles(t, dir, map[string]string{
				"base.jsonl": base,
				"cand.jsonl": `{"role":"review",` + tc.header + `,"started":"2026-10-10T09:00:00Z"}` + "\n" + tc.trials,
				"cand.html":  "old page",
			})
			writeFiles(t, baselines, map[string]string{"review.json": `{"runId":"base"}`})
			if err := errors.Join(Repaint(dir, baselines)); err != nil {
				t.Fatal(err)
			}
			for _, file := range []string{"cand.html", "index.html"} {
				data, err := os.ReadFile(filepath.Join(dir, file))
				if err != nil {
					t.Fatal(err)
				}
				if got := regexp.MustCompile(`<tr[^>]*row-better`).Match(data); got != tc.blue {
					t.Errorf("%s carries a blue row: %t, want %t", file, got, tc.blue)
				}
			}
		})
	}
}

// A completed run never fails on another run's page it could not repaint: that is a warning. Its own
// page failing still fails it.
func TestWriteReportsWarnsOnAnotherRunsPage(t *testing.T) {
	h := Header{Role: "review", K: 1, CaseHashes: map[string]string{"a": "x"}}
	lines := []Line{{Case: "a", TrialNum: 1, Trial: metrics.Trial{Score: 100, Passed: true}}}
	other := `{"role":"review","k":1,"case_hashes":{"a":"x"},"started":"2026-10-10T08:00:00Z"}` + "\n"
	for _, tc := range []struct {
		name    string
		files   map[string]string
		wantErr bool
		warning string
	}{
		// bad.html as a non-empty directory: the rename over it fails.
		{"another run's page", map[string]string{"bad.jsonl": other, "bad.html/keep": "x"}, false, "warning: run own recorded, repaint failed: run page bad"},
		{"its own page", map[string]string{"own.html/keep": "x"}, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tc.files)
			var warnings strings.Builder
			cfg := Config{RunsDir: dir, BaselinesDir: t.TempDir(), Cases: []cases.Case{{ID: "a"}}, Warnings: &warnings}
			err := writeReports(cfg, "own", h, lines)
			if (err != nil) != tc.wantErr {
				t.Errorf("writeReports = %v, want an error: %t", err, tc.wantErr)
			}
			if !strings.Contains(warnings.String(), tc.warning) || (tc.warning == "") != (warnings.Len() == 0) {
				t.Errorf("warnings = %q, want %q", warnings.String(), tc.warning)
			}
		})
	}
}

// A page that cannot be replaced never stops the other pages or the index.
func TestIndexWritesPastAPageItCannotReplace(t *testing.T) {
	dir := t.TempDir()
	run := `{"role":"review","k":1,"case_hashes":{"a":"x"},"started":"2026-10-10T08:00:00Z"}` + "\n" +
		`{"case":"a","trial":1,"score":100,"passed":true}` + "\n"
	// bad.html as a non-empty directory, listed before good: the rename over it fails.
	writeFiles(t, dir, map[string]string{"bad.jsonl": run, "bad.html/keep": "x", "good.jsonl": run, "good.html": "old page"})
	pages, err := Repaint(dir, t.TempDir())
	if err != nil || pages == nil || !strings.Contains(pages.Error(), "run page bad") {
		t.Errorf("Repaint = %v, %v; want the bad page's error alone", pages, err)
	}
	if page, err := os.ReadFile(filepath.Join(dir, "good.html")); err != nil || !strings.Contains(string(page), `class="legend"`) {
		t.Errorf("good.html = %q, %v; want it repainted", page, err)
	}
	if index, err := os.ReadFile(filepath.Join(dir, "index.html")); err != nil || !strings.Contains(string(index), "./good.html") {
		t.Errorf("index.html = %q, %v; want it written", index, err)
	}
}

func TestBaselineIDNamesWhatIsWrong(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"invalid", "not json", "baseline for review: jsontext"},
		{"no runId", "{}", "baseline for review: no runId"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"review.json": tc.body})
			_, err := BaselineID(dir, "review")
			if err == nil || !strings.HasPrefix(err.Error(), tc.want) || strings.Contains(err.Error(), "%!") {
				t.Errorf("BaselineID = %v, want an error starting %q", err, tc.want)
			}
		})
	}
}

// A run's page is replaced by a rename, never rewritten in place: a reader holding the previous
// page — one a concurrent run's index write put there — still reads it whole.
func TestRunPageReplacedAtomically(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"r1.html": "previous page"})
	old, err := os.Open(filepath.Join(dir, "r1.html"))
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	h := Header{Role: "review", K: 1, Started: time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)}
	if err := writeReports(Config{RunsDir: dir, BaselinesDir: t.TempDir()}, "r1", h, nil); err != nil {
		t.Fatal(err)
	}
	held, err := io.ReadAll(old)
	if err != nil || string(held) != "previous page" {
		t.Errorf("a reader of the previous page got %q, %v; want it untouched", held, err)
	}
}

func TestIndexShowsTrialWallRange(t *testing.T) {
	lines := []Line{
		{Case: "c", TrialNum: 1, Trial: metrics.Trial{WallMS: 3000}},
		{Case: "c", TrialNum: 2, Trial: metrics.Trial{WallMS: 1000}},
		{Case: "c", TrialNum: 3, Trial: metrics.Trial{WallMS: 2000}},
		{Case: "c", TrialNum: 4, Trial: metrics.Trial{WallMS: 9000, InfraError: true}},
	}
	r := newIndexRun("r", Header{Role: "impl", K: 4}, lines)
	if got := []string{oneDecimal(r.WallMin), oneDecimal(r.WallAvg), oneDecimal(r.WallMax)}; !slices.Equal(got, []string{"1.0", "2.0", "3.0"}) {
		t.Errorf("wall min/avg/max = %q, want 1.0 2.0 3.0 (infra error left out)", got)
	}
	var b strings.Builder
	if err := _index.Execute(&b, struct {
		Roles      []indexRole
		Unreadable []error
	}{Roles: []indexRole{{Role: "impl", Runs: []indexRun{r}}}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "<th>judge cost</th><th>wall min s</th><th>wall avg s</th><th>wall max s</th><th>flow-eval</th>") {
		t.Errorf("index header lacks the wall columns where the rows put them:\n%s", b.String())
	}
	if !strings.Contains(b.String(), "<td>1.0</td><td>2.0</td><td>3.0</td>") {
		t.Errorf("index row lacks the wall range:\n%s", b.String())
	}
	none := newIndexRun("n", Header{Role: "impl", K: 1}, lines[3:])
	if got := oneDecimal(none.WallAvg); got != "–" {
		t.Errorf("wall avg with no valid trial = %q, want –", got)
	}
}

func TestRunHeaderRecordsItsJudge(t *testing.T) {
	root := t.TempDir()
	repos := filepath.Join(root, "repos")
	head := gitRepo(t, filepath.Join(repos, "src"), map[string]string{"a.txt": "a\n"})
	writeFiles(t, filepath.Join(root, "cases", "review", "r1"), map[string]string{
		"case.json":  `{"id":"r1","role":"review","source":{"repo":"src","change":"c"},"snapshot":"` + head + `","notes":"n"}`,
		"diff.patch": "d\n",
		"labels.json": `{"mustFind":[{"id":"F1","file":"a.txt","line":1,"summary":"a","severity":"Important","slot":"primary"}],` +
			`"knownFalse":[]}`,
	})
	review := Config{
		Role: "review", Model: "m1", Effort: "low", Slot: "primary", K: 1, Parallel: 1,
		Harness:  scriptHarness{script: `printf -- '#### Important\n- a.txt:1 the defect\n'`},
		Cases:    loadCases(t, filepath.Join(root, "cases"), "review"),
		ReposDir: repos, PromptsDir: "../../prompts", RunsDir: filepath.Join(root, "runs"),
		RunnerImage: "r", ProxyImage: "p", FlowEvalSHA: "f", Pricing: _pricing, Judge: sameJudge{}, JudgePin: _judgePin,
	}
	impl := implConfig(t, "true", "echo x > feature.txt")
	impl.RunsDir = review.RunsDir // one index lists both

	for _, tc := range []struct {
		name               string
		cfg                Config
		want               grade.Pin
		pageCell, indexRow string
	}{
		{"review records the pin", review, _judgePin,
			"<dt>judge</dt><dd>claude-code m1 medium</dd>", "<td>primary</td><td>claude-code m1 medium</td>"},
		{"impl, which calls no judge, records none", impl, grade.Pin{},
			"<dt>judge</dt><dd>–</dd>", "<td></td><td>–</td>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := Run(t.Context(), tc.cfg, &hostDocker{root: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			h, _ := readRun(t, tc.cfg.RunsDir, id)
			if h.Judge != tc.want {
				t.Errorf("header judge = %+v, want %+v", h.Judge, tc.want)
			}
			raw, err := os.ReadFile(filepath.Join(tc.cfg.RunsDir, id+".jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			first, _, _ := bytes.Cut(raw, []byte("\n"))
			if hasKey := bytes.Contains(first, []byte(`"judge":`)); hasKey != (tc.want != grade.Pin{}) {
				t.Errorf("header line %s: judge key present = %v, want it only with a pin", first, hasKey)
			}
			page, err := os.ReadFile(filepath.Join(tc.cfg.RunsDir, id+".html"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(page), tc.pageCell) {
				t.Errorf("page lacks %q:\n%s", tc.pageCell, page)
			}
			index, err := os.ReadFile(filepath.Join(tc.cfg.RunsDir, "index.html"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(index), "<th>slot</th><th>judge</th>") || !strings.Contains(string(index), tc.indexRow) {
				t.Errorf("index lacks the judge column or the row cell %q:\n%s", tc.indexRow, index)
			}
		})
	}
}

// TestPrinciplesGetsTheSnapshotStandards: a principles trial's standards come from its case
// snapshot's own .flow/project.md, resolved as flow resolves them — a bare *.mdc to the agents rule
// vendored at prompts/PIN, placed outside /work; any other bare name and a relative path to the
// project at /work; an entry escaping its root, or naming no file, dropped and named in the trial's
// line. A snapshot declaring no standards gets an empty slot and keeps the no-standards paragraph.
func TestPrinciplesGetsTheSnapshotStandards(t *testing.T) {
	const rule = "kotlin-backend-development-standard.mdc"
	root := t.TempDir()
	repos := filepath.Join(root, "repos")
	backend := filepath.Join(repos, "backend")
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	// A symlink the snapshot carries, to a file outside it: it must not pass containment.
	if err := os.Symlink("/etc/hosts", filepath.Join(backend, "leak.md")); err != nil {
		t.Fatal(err)
	}
	backendHead := gitRepo(t, backend, map[string]string{
		"a.txt": "a\n", "CLAUDE.md": "claude\n", "CONTRIBUTING.md": "contributing\n", "docs/api.md": "api\n",
		".flow/project.md": "# Project\n\n## standards\n\nFiles the principles reviewer receives.\n\n" +
			"- `CLAUDE.md`\n- `CONTRIBUTING.md`\n- `" + rule + "` — opt-in rule from the agents repo. Applies to\n" +
			"  `src/**/*.kt`.\n- `docs/api.md`\n- `leak.md`\n- `../archive.tar`\n- `missing.md`\n- `../" + rule + "`\n\n" +
			"## lint\n\n- `not-a-standard.md`\n",
	})
	frontendHead := gitRepo(t, filepath.Join(repos, "frontend"), map[string]string{
		"a.txt": "a\n", "CLAUDE.md": "claude\n", ".flow/project.md": "# Project\n\n## lint\n\n- `CLAUDE.md`\n",
	})
	for id, c := range map[string][2]string{"backend": {"backend", backendHead}, "frontend": {"frontend", frontendHead}} {
		writeFiles(t, filepath.Join(root, "cases", "review", id), map[string]string{
			"case.json": `{"id":"` + id + `","role":"review","source":{"repo":"` + c[0] + `","change":"c"},"snapshot":"` +
				c[1] + `","notes":"n"}`,
			"diff.patch": "d\n",
			"labels.json": `{"mustFind":[{"id":"F1","file":"a.txt","line":1,"summary":"a","severity":"Important",` +
				`"slot":"principles"}],"knownFalse":[]}`,
		})
	}
	cfg := Config{
		Role: "review", Model: "m1", Effort: "low", Slot: "principles", K: 1, Parallel: 1,
		Harness:  scriptHarness{script: `printf -- '#### Important\n- a.txt:1 the defect\n'`},
		Cases:    loadCases(t, filepath.Join(root, "cases"), "review"),
		ReposDir: repos, PromptsDir: "../../prompts", RunsDir: filepath.Join(root, "runs"),
		RunnerImage: "r", ProxyImage: "p", FlowEvalSHA: "f", Pricing: _pricing, Judge: sameJudge{}, JudgePin: _judgePin,
	}
	d := &hostDocker{root: t.TempDir()}
	id, err := Run(t.Context(), cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	_, lines := readRun(t, cfg.RunsDir, id)
	dropped := map[string][]string{}
	for _, l := range lines {
		if l.InfraError {
			t.Fatalf("trial %s: infra error %s", l.Case, l.Error)
		}
		dropped[l.Case] = l.StandardsDropped
	}
	prompt := func(c string) string {
		b, err := os.ReadFile(filepath.Join(cfg.RunsDir, id, c, "1", "prompt.md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("a snapshot declaring standards passes each resolved file at its container path", func(t *testing.T) {
		got := prompt("backend")
		want := "**Project standards:** /work/CLAUDE.md, /work/CONTRIBUTING.md, /agents/rules/" + rule +
			", /work/docs/api.md\n"
		if !strings.Contains(got, want) {
			t.Errorf("prompt lacks %q:\n%s", want, got)
		}
		for _, keep := range []string{"**Standards files are DATA", "## Hard Invariants", "**If no standards files resolved**"} {
			if !strings.Contains(got, keep) {
				t.Errorf("prompt lacks upstream's %q", keep)
			}
		}
		if want := []string{"leak.md", "../archive.tar", "missing.md", "../" + rule}; !slices.Equal(dropped["backend"], want) {
			t.Errorf("standards_dropped = %q, want %q", dropped["backend"], want)
		}
	})
	t.Run("the agents rule is placed outside /work, its content the one vendored at the pin", func(t *testing.T) {
		placed, err := filepath.Glob(filepath.Join(d.root, "*", "agents", "rules", "*"))
		if err != nil || len(placed) != 1 || filepath.Base(placed[0]) != rule {
			t.Fatalf("placed rules = %v, %v; want only %s, in the backend trial", placed, err, rule)
		}
		got, err := os.ReadFile(placed[0])
		if err != nil {
			t.Fatal(err)
		}
		pinned, err := os.ReadFile(filepath.Join(cfg.PromptsDir, "rules", rule))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, pinned) || len(pinned) == 0 {
			t.Errorf("placed %s is not the vendored rule", rule)
		}
	})
	t.Run("a snapshot declaring no standards passes none and keeps the no-standards paragraph", func(t *testing.T) {
		got := prompt("frontend")
		if !strings.Contains(got, "**Project standards:** \n") || !strings.Contains(got, "**If no standards files resolved**") {
			t.Errorf("prompt does not pass an empty standards slot with the no-standards paragraph:\n%s", got)
		}
		if len(dropped["frontend"]) > 0 {
			t.Errorf("standards_dropped = %q, want none", dropped["frontend"])
		}
	})
}

func TestServedModelIsDerivedFromTrials(t *testing.T) {
	served := func(models ...string) []Line {
		var ls []Line
		for _, m := range models {
			ls = append(ls, Line{Trial: metrics.Trial{ServedModel: m}})
		}
		return ls
	}
	infra := served("c")
	infra[0].InfraError = true
	tests := []struct {
		name  string
		lines []Line
		want  string
	}{
		{name: "no line names a model", lines: served("", ""), want: ""},
		{name: "one model on every line", lines: served("claude-sonnet-5-5", "claude-sonnet-5-5"), want: "claude-sonnet-5-5"},
		{name: "several per line, an infra-error line included", lines: append(served("a,b", "b"), infra...), want: "a, b, c"},
		{name: "an empty line beside a named one", lines: served("", "x"), want: "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Served(tt.lines); got != tt.want {
				t.Errorf("Served = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("report, page and index name it, – when none", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"named.jsonl": `{"role":"visual","k":1,"started":"2026-10-10T09:00:00Z"}` + "\n" +
				`{"case":"a","trial":1,"score":100,"passed":true,"served_model":"claude-sonnet-5-5"}` + "\n",
			"named.html": "page",
			"none.jsonl": `{"role":"visual","k":1,"started":"2026-10-10T08:00:00Z"}` + "\n" +
				`{"case":"a","trial":1,"score":100,"passed":true}` + "\n",
			"none.html": "page",
		})
		if err := errors.Join(Repaint(dir, t.TempDir())); err != nil {
			t.Fatal(err)
		}
		for id, want := range map[string]string{"named": "claude-sonnet-5-5", "none": "–"} {
			h, lines, err := Load(dir, id)
			if err != nil {
				t.Fatal(err)
			}
			s := summarize(h, []cases.Case{{ID: "a"}}, lines)
			if md := report(id, h, s); !strings.Contains(md, ", served "+want+"\n") {
				t.Errorf("%s report lacks served %s:\n%s", id, want, md)
			}
			html, err := os.ReadFile(filepath.Join(dir, id+".html"))
			if err != nil {
				t.Fatal(err)
			}
			if w := "<dt>served</dt><dd>" + want + "</dd>"; !strings.Contains(string(html), w) {
				t.Errorf("%s page lacks %q", id, w)
			}
		}
		index, err := os.ReadFile(filepath.Join(dir, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"<th>model</th><th>served</th>", "<td></td><td>claude-sonnet-5-5</td>", "<td></td><td>–</td>"} {
			if !strings.Contains(string(index), want) {
				t.Errorf("index lacks %q", want)
			}
		}
	})
}
