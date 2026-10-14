package main

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tweety53/flow-eval/internal/harness"
	"github.com/tweety53/flow-eval/internal/metrics"
	"github.com/tweety53/flow-eval/internal/run"
)

// buildBinary compiles flow-eval into a temp dir so tests exercise the real process: exit codes and
// stderr are what an operator sees.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "flow-eval")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runBinary(t *testing.T, bin string, args ...string) (exit int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	ee, isExit := errors.AsType[*exec.ExitError](err)
	switch {
	case err == nil:
	case isExit:
		exit = ee.ExitCode()
	default:
		t.Fatalf("run %v: %v", args, err)
	}
	return exit, out.String(), errb.String()
}

func TestUnknownCommandExits2(t *testing.T) {
	bin := buildBinary(t)
	for _, args := range [][]string{{"nope"}, {}} {
		exit, _, stderr := runBinary(t, bin, args...)
		if exit != 2 {
			t.Errorf("flow-eval %v: exit %d, want 2", args, exit)
		}
		if !strings.Contains(stderr, "usage: flow-eval") {
			t.Errorf("flow-eval %v: stderr lacks usage:\n%s", args, stderr)
		}
	}
}

func TestHelpListsCommands(t *testing.T) {
	bin := buildBinary(t)
	exit, stdout, _ := runBinary(t, bin, "--help")
	if exit != 0 {
		t.Fatalf("flow-eval --help: exit %d, want 0", exit)
	}
	for _, c := range []string{"run", "compare", "baseline", "case", "prompts"} {
		if !strings.Contains(stdout, "\n  "+c+" ") {
			t.Errorf("--help does not list %q:\n%s", c, stdout)
		}
	}
}

func TestCaseValidateExitCodes(t *testing.T) {
	bin := buildBinary(t)
	const td = "../../internal/cases/testdata/"
	// A source checkout holding a mockup, so case add visual can succeed end to end.
	src := t.TempDir()
	png := filepath.Join(src, "home.png")
	if err := os.WriteFile(png, []byte("\x89PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", src, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	drafts := t.TempDir()
	for _, tc := range []struct {
		args       []string
		exit       int
		wantStdout string
		wantStderr string
	}{
		{[]string{"case", "validate", "--root", td + "valid"}, 0, "3 cases valid", ""},
		{[]string{"case", "validate", "--root", td + "valid", "--role", "impl"}, 0, "1 cases valid", ""},
		{[]string{"case", "validate", "--root", td + "missing-asset"}, 1, "", "missing asset diff.patch"},
		{[]string{"case", "validate", "--root", td + "unknown-role"}, 1, "", `role "foo"`},
		{[]string{"case", "validate", "--root", td + "valid", "--role", "foo"}, 2, "", `unknown role "foo"`},
		{[]string{"case", "validate", "--root", td + "absent"}, 2, "", "absent"},
		{[]string{"case", "validate", "--bogus"}, 2, "", "-bogus"},
		{[]string{"case", "validate", "--bogus", "-h"}, 2, "", "usage: flow-eval case validate"},
		{[]string{"case", "validate", "-h"}, 0, "usage: flow-eval case validate", ""},
		{[]string{"case", "validate", "extra"}, 2, "", "usage: flow-eval case validate"},
		{[]string{"case"}, 2, "", "usage: flow-eval case"},
		{[]string{"case", "nope"}, 2, "", "usage: flow-eval case"},
		{[]string{"case", "add"}, 2, "", "usage: flow-eval case add"},
		{[]string{"case", "add", "nope"}, 2, "", "usage: flow-eval case add"},
		{[]string{"case", "add", "impl", "-h"}, 0, "usage: flow-eval case add", ""},
		{[]string{"case", "add", "impl", "--repo", src, "--change", "c"}, 2, "", "usage: flow-eval case add"},
		{[]string{"case", "add", "review", "--repo", src}, 2, "", "usage: flow-eval case add"},
		{[]string{"case", "add", "visual", "--id", "v"}, 2, "", "usage: flow-eval case add"},
		{[]string{"case", "add", "review", "--repo", src, "--change", "c", "--root", drafts}, 2, "", "ledger.md"},
		{[]string{"case", "add", "visual", "--region", "1,2,3,4"}, 2, "", "a --region must follow its --defect"},
		{[]string{"case", "add", "visual", "--defect", "x", "--region", "1,2"}, 2, "", "not x,y,w,h"},
		{[]string{"case", "add", "visual", "--id", "v", "--mockup", png, "--capture", png, "--verdict", "maybe",
			"--root", drafts}, 2, "", `verdict "maybe"`},
		{[]string{"case", "add", "visual", "--id", "v", "--mockup", png, "--capture", png, "--verdict", "mismatch",
			"--defect", "title 2px low", "--region", "1,2,3,4", "--root", drafts}, 0, "edit labels, then commit", ""},
		{[]string{"case", "validate", "--root", drafts}, 0, "1 cases valid", ""},
	} {
		exit, stdout, stderr := runBinary(t, bin, tc.args...)
		if exit != tc.exit || !strings.Contains(stdout, tc.wantStdout) || !strings.Contains(stderr, tc.wantStderr) {
			t.Errorf("flow-eval %v: exit %d (want %d)\nstdout: %s\nstderr: %s", tc.args, exit, tc.exit, stdout, stderr)
		}
	}
}

func TestPromptsSyncExitCodes(t *testing.T) {
	bin := buildBinary(t)
	notRepo := t.TempDir()
	for _, tc := range []struct {
		args       []string
		exit       int
		wantStderr string
	}{
		{[]string{"prompts"}, 2, "usage: flow-eval prompts sync"},
		{[]string{"prompts", "nope"}, 2, "usage: flow-eval prompts sync"},
		{[]string{"prompts", "sync"}, 2, "usage: flow-eval prompts sync"},
		{[]string{"prompts", "sync", "--agents", notRepo}, 2, "usage: flow-eval prompts sync"},
		{[]string{"prompts", "sync", "--agents", notRepo, "--sha", "deadbeef", "--dir", t.TempDir()}, 2, "git rev-parse"},
	} {
		exit, _, stderr := runBinary(t, bin, tc.args...)
		if exit != tc.exit || !strings.Contains(stderr, tc.wantStderr) {
			t.Errorf("flow-eval %v: exit %d (want %d)\nstderr: %s", tc.args, exit, tc.exit, stderr)
		}
	}
	if exit, stdout, _ := runBinary(t, bin, "prompts", "sync", "-h"); exit != 0 || !strings.Contains(stdout, "usage: flow-eval prompts sync") {
		t.Errorf("flow-eval prompts sync -h: exit %d, stdout %q; want 0 and the usage on stdout", exit, stdout)
	}
}

func TestRunRefusesBeforeAnyContainer(t *testing.T) {
	bin := buildBinary(t)
	// A docker on PATH that records being called, so a refusal is proven to precede every container.
	fakeBin := t.TempDir()
	called := filepath.Join(t.TempDir(), "docker-called")
	script := "#!/bin/sh\ntouch " + called + "\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + fakeBin + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	full := []string{"run", "--role", "visual", "--harness", "codex", "--model", "m", "--effort", "low"}
	for _, tc := range []struct {
		name       string
		args, env  []string
		wantStderr string
	}{
		{"no flags", []string{"run"}, nil, "usage: flow-eval run"},
		{"unknown role", []string{"run", "--role", "x", "--harness", "codex", "--model", "m", "--effort", "e"}, nil, "usage: flow-eval run"},
		{"unknown harness", []string{"run", "--role", "impl", "--harness", "x", "--model", "m", "--effort", "e"}, nil, "usage: flow-eval run"},
		{"harness credential missing", full, nil, "OPENAI_API_KEY"},
		{"judge credential missing", full, []string{"OPENAI_API_KEY=k"}, "CLAUDE_CODE_OAUTH_TOKEN"},
		// muse-code's login lives in its volume, so no credential is asked of it; only the judge's.
		{"muse-code needs no credential", []string{"run", "--role", "visual", "--harness", "muse-code", "--model", "m", "--effort", "low"}, nil, "CLAUDE_CODE_OAUTH_TOKEN"},
		{"unknown case", append(full, "--root", t.TempDir(), "--cases", "nope"), []string{"OPENAI_API_KEY=k", "ANTHROPIC_API_KEY=k"}, "nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			cmd := exec.Command(bin, tc.args...)
			cmd.Env, cmd.Stderr = append(slices.Clone(env), tc.env...), &stderr
			err := cmd.Run()
			if ee, ok := errors.AsType[*exec.ExitError](err); !ok || ee.ExitCode() != 2 || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("err %v, stderr %q; want exit 2 naming %q", err, stderr.String(), tc.wantStderr)
			}
			if _, err := os.Stat(called); err == nil {
				t.Error("docker was called before the refusal")
			}
		})
	}
}

// A muse-code run checks its volume holds the binary and the login before any trial, by running the
// local runner image, never a pulled one, with the harness's mounts, and exits 2 naming the setup
// when it does not, or naming the image when docker cannot run it; once the check passes, each
// trial container gets those mounts.
func TestMuseVolumeCheckedAndMountedIntoTrials(t *testing.T) {
	bin := buildBinary(t)
	for _, tc := range []struct {
		name     string
		probe    int // the probe container's exit: 2 is the check's own, 125 docker's
		wantExit int
		stderr   string
	}{
		{"volume not set up", 2, 2, "to hold the binary and the login: run the setup in docs/muse-code.md"},
		{"runner image missing", 125, 2, "cannot run the runner image flow-eval-runner"},
		{"volume set up", 0, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A docker on PATH that logs each call; the probe, the one --network none run, exits tc.probe
			// and every other call succeeds with no output.
			fakeBin, log, runs := t.TempDir(), filepath.Join(t.TempDir(), "docker.log"), filepath.Join(t.TempDir(), "runs")
			script := fmt.Sprintf("#!/bin/sh\necho \"$*\" >> %s\ncase \"$*\" in \"run --rm --network none \"*) exit %d;; esac\n",
				log, tc.probe)
			if err := os.WriteFile(filepath.Join(fakeBin, "docker"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd := exec.Command(bin, "run", "--role", "visual", "--harness", "muse-code", "--model", "muse-spark-1.3",
				"--effort", "low", "-k", "1", "--root", "../../internal/cases/testdata/valid", "--runs", runs,
				"--prompts", "../../prompts", "--pricing", "../../pricing.json", "--judge", "../../judge.json")
			cmd.Env = []string{"PATH=" + fakeBin + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "ANTHROPIC_API_KEY=k"}
			cmd.Stderr = &stderr
			err := cmd.Run()
			exit := 0
			if ee, ok := errors.AsType[*exec.ExitError](err); ok {
				exit = ee.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if exit != tc.wantExit {
				t.Fatalf("exit %d, want %d; stderr %s", exit, tc.wantExit, stderr.String())
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			var mounts string
			for _, m := range harness.MuseMounts() {
				mounts += "--mount " + m + " "
			}
			probe := "run --rm --network none --pull never " + mounts + "--entrypoint sh flow-eval-runner -c " +
				harness.MuseReady
			if !strings.Contains(string(calls), probe+"\n") {
				t.Errorf("no probe %q in docker calls:\n%s", probe, calls)
			}
			trialStarted := regexp.MustCompile(`(?m)^run -d --name fe-\S+-trial .*` + regexp.QuoteMeta(mounts)).Match(calls)
			if tc.wantExit == 2 {
				if !strings.Contains(stderr.String(), tc.stderr) {
					t.Errorf("stderr does not say %q: %s", tc.stderr, stderr.String())
				}
				if _, err := os.Stat(runs); err == nil {
					t.Error("a run was recorded although the volume is not set up")
				}
			} else if !trialStarted {
				t.Errorf("no trial container started with the muse mounts:\n%s", calls)
			}
		})
	}
}

// With both Claude credentials set, a trial gets only the first, the subscription token.
func TestOnlyTheFirstSetCredentialEntersATrial(t *testing.T) {
	creds := _harnesses["claude-code"].creds
	for _, tc := range []struct {
		name string
		set  map[string]string
		want []string
	}{
		{"both set", map[string]string{"CLAUDE_CODE_OAUTH_TOKEN": "o", "ANTHROPIC_API_KEY": "a"}, []string{"CLAUDE_CODE_OAUTH_TOKEN"}},
		{"only the API key", map[string]string{"CLAUDE_CODE_OAUTH_TOKEN": "", "ANTHROPIC_API_KEY": "a"}, []string{"ANTHROPIC_API_KEY"}},
		{"none", map[string]string{"CLAUDE_CODE_OAUTH_TOKEN": "", "ANTHROPIC_API_KEY": ""}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.set {
				t.Setenv(k, v)
			}
			if got := firstCredential(creds); !slices.Equal(got, tc.want) {
				t.Errorf("firstCredential = %v, want %v", got, tc.want)
			}
		})
	}
}

// writeRun writes runs/<id>.jsonl with case a scoring score on each of k trials, at flow-eval sha
// sha-<id>.
func writeRun(t *testing.T, runsDir, id string, k int, score float64) {
	t.Helper()
	h, err := json.Marshal(run.Header{Role: "review", Slot: "primary", K: k, PromptPin: "pin",
		CaseSetHash: "set", CaseHashes: map[string]string{"a": "h"}, FlowEvalSHA: "sha-" + id})
	if err != nil {
		t.Fatal(err)
	}
	body := append(h, '\n')
	for i := range k {
		l, err := json.Marshal(run.Line{Case: "a", TrialNum: i + 1, Trial: metrics.Trial{Score: score, CostUSD: 1}})
		if err != nil {
			t.Fatal(err)
		}
		body = append(append(body, l...), '\n')
	}
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runsDir, id+".jsonl"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompareAndBaselineExitCodes(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	runs, baselines := filepath.Join(dir, "runs"), filepath.Join(dir, "baselines")
	writeRun(t, runs, "base", 3, 60)
	writeRun(t, runs, "same", 3, 58)
	writeRun(t, runs, "worse", 3, 40)
	writeRun(t, runs, "better", 3, 90)
	writeRun(t, runs, "short", 2, 60)
	writeRun(t, runs, "shortk", 3, 60)
	// shortk claims k=3 but loses its last trial line.
	data, err := os.ReadFile(filepath.Join(runs, "shortk.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(data), "\n")
	if err := os.WriteFile(filepath.Join(runs, "shortk.jsonl"), []byte(strings.Join(lines[:3], "")), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs := []string{"--runs", runs, "--baselines", baselines}
	for _, tc := range []struct {
		args []string
		exit int
	}{
		{[]string{"compare", "same"}, 2}, // no baseline recorded yet
		{[]string{"baseline"}, 2},
		{[]string{"baseline", "set"}, 2},
		{[]string{"baseline", "set", "nope"}, 2},
		{[]string{"baseline", "set", "base"}, 0},
		{[]string{"compare"}, 2},
		{[]string{"compare", "nope"}, 2},
		{[]string{"compare", "same"}, 0},
		{[]string{"compare", "worse"}, 1},
		{[]string{"compare", "better"}, 3},
		{[]string{"compare", "better", "--margin", "50"}, 0},
		{[]string{"compare", "shortk"}, 2},
		{[]string{"compare", "--baseline", "short", "same"}, 2}, // k 3 against 2
		{[]string{"compare", "same", "--margin", "0"}, 2},
	} {
		exit, _, stderr := runBinary(t, bin, append(tc.args, dirs...)...)
		if exit != tc.exit {
			t.Errorf("flow-eval %v: exit %d, want %d\nstderr: %s", tc.args, exit, tc.exit, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(runs, "worse-vs-base.md")); err != nil {
		t.Errorf("compare wrote no report: %v", err)
	}
	// Runs of different flow-eval shas are compared, never refused, and the difference is printed.
	// Review runs recording no judge are compared too, with the judge warning on stderr.
	exit, stdout, stderr := runBinary(t, bin, append([]string{"compare", "same"}, dirs...)...)
	if want := "flow-eval sha-same vs sha-base — DIFFER"; exit != 0 || !strings.Contains(stdout, want) {
		t.Errorf("compare same: exit %d, stdout %q; want 0 and %q", exit, stdout, want)
	}
	if want := "warning: judge – vs – — UNRECORDED"; !strings.Contains(stderr, want) {
		t.Errorf("compare same: stderr %q; want %q", stderr, want)
	}
	// Runs under different prompt pins are compared too, with the pin warning on stderr.
	writeRun(t, runs, "repinned", 3, 58)
	if data, err = os.ReadFile(filepath.Join(runs, "repinned.jsonl")); err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`"prompt_pin":"pin"`), []byte(`"prompt_pin":"pin2"`), 1)
	if err := os.WriteFile(filepath.Join(runs, "repinned.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	exit, _, stderr = runBinary(t, bin, append([]string{"compare", "repinned"}, dirs...)...)
	if want := "warning: prompt pin pin2 vs pin — DIFFER"; exit != 0 || !strings.Contains(stderr, want) {
		t.Errorf("compare repinned: exit %d, stderr %q; want 0 and %q", exit, stderr, want)
	}
	if b, err := os.ReadFile(filepath.Join(baselines, "review-primary.json")); err != nil || !strings.Contains(string(b), `"runId":"base"`) {
		t.Errorf("baselines/review-primary.json = %s, %v", b, err)
	}
	// Recording the baseline repaints the index: a run scoring above it is blue.
	if b, err := os.ReadFile(filepath.Join(runs, "index.html")); err != nil || !strings.Contains(string(b), `<tr class="row-better"><td>better `) {
		t.Errorf("index after baseline set = %v, does not paint the better run blue", err)
	}
}

// Once baselines/<role>.json names the run, a failed repaint is a warning, never a failed exit.
func TestBaselineSetWarnsOnAFailedRepaint(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	runs, baselines := filepath.Join(dir, "runs"), filepath.Join(dir, "baselines")
	writeRun(t, runs, "base", 1, 60)
	// base.html as a non-empty directory: the repaint cannot replace it.
	if err := os.MkdirAll(filepath.Join(runs, "base.html", "keep"), 0o755); err != nil {
		t.Fatal(err)
	}
	exit, _, stderr := runBinary(t, bin, "baseline", "set", "base", "--runs", runs, "--baselines", baselines)
	if exit != 0 || !strings.Contains(stderr, "warning: baseline recorded, repaint failed: run page base") {
		t.Errorf("baseline set: exit %d, stderr %q; want 0 and a repaint warning", exit, stderr)
	}
	if b, err := os.ReadFile(filepath.Join(baselines, "review-primary.json")); err != nil || !strings.Contains(string(b), `"runId":"base"`) {
		t.Errorf("baselines/review-primary.json = %s, %v", b, err)
	}
}

// A SIGTERM — a CI cancel, `timeout`, `kill` — cancels a run like Ctrl+C does, so its deferred
// teardown removes the trials' containers instead of the process dying with them running.
func TestSIGTERMCancelsARun(t *testing.T) {
	ctx, stop := signal.NotifyContext(t.Context(), stopSignals...)
	defer stop()
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM did not cancel the run context")
	}
}

// regrade exits 2, naming why, before any judge call: without a run id, without the judge's
// credential, and on a run that does not exist.
func TestRegradeExitCodes(t *testing.T) {
	bin := buildBinary(t)
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	dirs := []string{"--root", "../../cases", "--judge", "../../judge.json", "--pricing", "../../pricing.json",
		"--runs", t.TempDir()}
	for _, tc := range []struct {
		name       string
		args, env  []string
		wantStderr string
	}{
		{"no run id", append([]string{"regrade"}, dirs...), []string{"ANTHROPIC_API_KEY=k"}, "usage: flow-eval regrade"},
		{"judge credential missing", append([]string{"regrade", "r1"}, dirs...), nil, "CLAUDE_CODE_OAUTH_TOKEN"},
		{"unknown run", append([]string{"regrade", "r1"}, dirs...), []string{"ANTHROPIC_API_KEY=k"}, "r1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			cmd := exec.Command(bin, tc.args...)
			cmd.Env, cmd.Stderr = append(slices.Clone(env), tc.env...), &stderr
			err := cmd.Run()
			if ee, ok := errors.AsType[*exec.ExitError](err); !ok || ee.ExitCode() != 2 || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("err %v, stderr %q; want exit 2 naming %q", err, stderr.String(), tc.wantStderr)
			}
		})
	}
}
