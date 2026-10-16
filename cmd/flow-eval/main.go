// Command flow-eval runs flow's agent roles on recorded cases in isolated containers and decides
// whether a candidate model, effort or harness is non-inferior to the role's baseline.
//
// Exit codes shared by every subcommand: 0 success, 2 usage or infrastructure error; a subcommand
// documents any other code it returns.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tweety53/flow-eval/internal/cases"
	"github.com/tweety53/flow-eval/internal/compare"
	"github.com/tweety53/flow-eval/internal/grade"
	"github.com/tweety53/flow-eval/internal/harness"
	"github.com/tweety53/flow-eval/internal/metrics"
	"github.com/tweety53/flow-eval/internal/mine"
	"github.com/tweety53/flow-eval/internal/prompts"
	"github.com/tweety53/flow-eval/internal/run"
	"github.com/tweety53/flow-eval/internal/sandbox"
)

type command struct {
	name    string
	summary string
	run     func(args []string) int
}

// commands is the single list the router and the usage text both read, in usage order.
var commands = []command{
	{name: "run", summary: "run one role on its case set for k trials", run: runCmd},
	{
		name:    "compare",
		summary: "decide whether a candidate run is non-inferior to the role's baseline",
		run:     compareCmd,
	},
	{
		name:    "baseline",
		summary: "baseline set <run-id>: record a run as its role's baseline",
		run:     baselineCmd,
	},
	{
		name:    "regrade",
		summary: "regrade <run-id>…: grade stored review runs again against the current labels and judge",
		run:     regradeCmd,
	},
	{name: "case", summary: "case add|validate: add and check eval cases", run: caseCmd},
	{
		name:    "prompts",
		summary: "prompts sync: vendor the flow role prompts from a pinned agents sha",
		run:     promptsCmd,
	},
}

func main() {
	os.Exit(dispatch(os.Args[1:]))
}

func dispatch(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "-h", "-help", "--help", "help":
		usage(os.Stdout)
		return 0
	}
	for _, c := range commands {
		if c.name == args[0] {
			return c.run(args[1:])
		}
	}
	fmt.Fprintf(os.Stderr, "flow-eval: unknown command %q\n", args[0])
	usage(os.Stderr)
	return 2
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: flow-eval <command> [flags]")
	fmt.Fprintln(w, "\ncommands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-9s %s\n", c.name, c.summary)
	}
}

// _harnesses are the harnesses run takes, each with the environment variables that authenticate
// it, any one of which is enough, in the order a trial is given the first one set; or else the Docker
// volume holding its login and binary, which each trial mounts, and the shell script that passes in
// the runner image with those mounts once the volume is set up, and exits 2 until it is.
var _harnesses = map[string]struct {
	h      harness.Harness
	creds  []string
	volume string
	mounts []string
	ready  string
}{
	"claude-code": {h: harness.Claude{}, creds: []string{"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_API_KEY"}},
	"codex":       {h: harness.Codex{}, creds: []string{"OPENAI_API_KEY"}},
	"muse-code":   {h: harness.Muse{}, volume: harness.MuseVolume, mounts: harness.MuseMounts(), ready: harness.MuseReady},
}

// firstCredential returns the first of names set in the environment, as a one-name list, or nil when
// none is: one credential authenticates a harness, so no second secret enters a trial.
func firstCredential(names []string) []string {
	i := slices.IndexFunc(names, func(n string) bool { return os.Getenv(n) != "" })
	if i < 0 {
		return nil
	}
	return names[i : i+1]
}

// stopSignals cancel a run, so its trials' containers, networks and volumes are removed: Ctrl+C at a
// terminal, and SIGTERM from a CI cancel, `timeout` or `kill`.
var stopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// runCmd exits 0 when every trial is recorded (infra-error trials included), 2 on a usage error,
// missing credentials, an invalid or unknown case, or a run that cannot proceed.
func runCmd(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	role := fs.String("role", "", "role to run: impl, review or visual")
	harnessName := fs.String("harness", "", "harness: claude-code, codex or muse-code")
	model := fs.String("model", "", "model the harness runs")
	effort := fs.String("effort", "", "effort the harness runs at")
	k := fs.Int("k", 3, "trials per case")
	only := fs.String("cases", "", "comma-separated case ids to run; all of the role when empty")
	parallel := fs.Int("parallel", 2, "trials run at once")
	slot := fs.String("slot", "primary", "reviewer slot whose prompt a review run uses, and whose must-finds it is graded on")
	noFlow := fs.Bool("no-flow", false, "run the role with no flow prompt, rules or tooling: the harness's own behaviour (review: claude-code's /code-review, or a plain review request on muse-code)")
	root := fs.String("root", "cases", "cases directory")
	repos := fs.String("repos", "..", "directory holding each case's source checkout as <repos>/<source.repo>")
	runsDir := fs.String("runs", "runs", "directory the run records are written to")
	baselines := fs.String("baselines", "baselines", "directory holding baselines/<role>[-no-flow][-<slot>].json, read to paint the index")
	promptsDir := fs.String("prompts", "prompts", "vendored prompts directory")
	pricingPath := fs.String("pricing", "pricing.json", "model prices")
	judgePath := fs.String("judge", "judge.json", "judge pin")
	runner := fs.String("runner-image", "flow-eval-runner", "trial runner image")
	proxy := fs.String("proxy-image", "flow-eval-proxy", "egress proxy image")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: flow-eval run --role R --harness H --model M --effort E [-k 3] "+
			"[--cases id,…] [--parallel 2] [--slot primary] [--no-flow]")
		fs.PrintDefaults()
	}
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	hr, known := _harnesses[*harnessName]
	if fs.NArg() > 0 || !slices.Contains([]string{"impl", "review", "visual"}, *role) || !known ||
		*model == "" || *effort == "" || *k < 1 || *parallel < 1 {
		fs.Usage()
		return 2
	}
	if *noFlow && *role == "review" {
		if _, err := run.NoFlowReviewTarget(*harnessName); err != nil {
			fmt.Fprintf(os.Stderr, "flow-eval run: %v\n", err)
			return 2
		}
	}
	creds := firstCredential(hr.creds)
	if len(hr.creds) > 0 && len(creds) == 0 {
		fmt.Fprintf(os.Stderr, "flow-eval run: %s needs one of %s set\n", *harnessName, strings.Join(hr.creds, ", "))
		return 2
	}
	judgeCreds := _harnesses["claude-code"].creds
	if *role != "impl" && len(firstCredential(judgeCreds)) == 0 {
		fmt.Fprintf(os.Stderr, "flow-eval run: the %s grader's judge needs one of %s set\n", *role, strings.Join(judgeCreds, ", "))
		return 2
	}

	selected, err := selectCases(*root, *role, *only)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval run: %v\n", err)
		return 2
	}
	pricing, judge, sha, err := grading(*pricingPath, *judgePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval run: %v\n", err)
		return 2
	}
	// Checked here, not left to the first trial: docker would create the volume empty, and every
	// trial would then fail on its mount. The volume must exist before the probe mounts it, which
	// would create it.
	if hr.volume != "" {
		if out, err := exec.Command("docker", "volume", "inspect", hr.volume).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "flow-eval run: %s needs the docker volume %s: run the setup in docs/muse-code.md (%v: %s)\n",
				*harnessName, hr.volume, err, strings.TrimSpace(string(out)))
			return 2
		}
		// --pull never: the probe runs the image the trials will, never one pulled from a registry.
		probe := []string{"run", "--rm", "--network", "none", "--pull", "never"}
		for _, m := range hr.mounts {
			probe = append(probe, "--mount", m)
		}
		probe = append(probe, "--entrypoint", "sh", *runner, "-c", hr.ready)
		out, err := exec.Command("docker", probe...).CombinedOutput()
		if ee, ok := errors.AsType[*exec.ExitError](err); ok && ee.ExitCode() == 2 { // the ready check's own exit
			fmt.Fprintf(os.Stderr, "flow-eval run: %s needs the docker volume %s to hold the binary and the login: run the setup in docs/muse-code.md (%s)\n",
				*harnessName, hr.volume, strings.TrimSpace(string(out)))
			return 2
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "flow-eval run: cannot run the runner image %s to check the docker volume %s; is the image built? (%v: %s)\n",
				*runner, hr.volume, err, strings.TrimSpace(string(out)))
			return 2
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	id, err := run.Run(ctx, run.Config{
		Role: *role, Model: *model, Effort: *effort, Slot: *slot, Harness: hr.h, K: *k, Parallel: *parallel,
		Cases: selected, ReposDir: *repos, PromptsDir: *promptsDir, RunsDir: *runsDir, BaselinesDir: *baselines,
		RunnerImage: *runner, ProxyImage: *proxy, Credentials: creds, Mounts: hr.mounts,
		FlowEvalSHA: sha, Pricing: pricing, Judge: judge, JudgePin: judge.Pin,
		Warnings: os.Stderr, NoFlow: *noFlow,
	}, run.DockerHost{CLI: sandbox.DockerCLI{}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval run: %v\n", err)
		return 2
	}
	fmt.Printf("run %s: %s/%s.jsonl, %s/%s.md, %s/%s.html\n", id, *runsDir, id, *runsDir, id, *runsDir, id)
	if index, err := filepath.Abs(filepath.Join(*runsDir, "index.html")); err == nil {
		fmt.Printf("index: %s\n", index)
	}
	return 0
}

// grading loads what grading a trial needs and a run records: the model prices, the judge, and the
// flow-eval sha checked out.
func grading(pricingPath, judgePath string) (metrics.Pricing, grade.CLIJudge, string, error) {
	pricing, err := metrics.LoadPricing(pricingPath)
	if err != nil {
		return nil, grade.CLIJudge{}, "", err
	}
	judge, err := grade.LoadJudge(judgePath)
	if err != nil {
		return nil, grade.CLIJudge{}, "", err
	}
	sha, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, grade.CLIJudge{}, "", fmt.Errorf("flow-eval git sha: %w", err)
	}
	return pricing, judge, strings.TrimSpace(string(sha)), nil
}

// selectCases loads role's cases under root and keeps those named in the comma-separated only,
// every one when only is empty. An invalid case or an unknown id is an error.
func selectCases(root, role, only string) ([]cases.Case, error) {
	loaded, invalid, err := cases.LoadAll(root, role)
	if err != nil {
		return nil, err
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("invalid cases: %w", errors.Join(invalid...))
	}
	if only == "" {
		if len(loaded) == 0 {
			return nil, fmt.Errorf("no %s cases under %s", role, root)
		}
		return loaded, nil
	}
	var selected []cases.Case
	for id := range strings.SplitSeq(only, ",") {
		i := slices.IndexFunc(loaded, func(c cases.Case) bool { return c.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("no %s case %q under %s", role, id, root)
		}
		selected = append(selected, loaded[i])
	}
	return selected, nil
}

// parseWithID parses args into fs and takes one positional run id, before or after the flags.
// ok is false when the subcommand must exit with code.
func parseWithID(fs *flag.FlagSet, args []string) (id string, code int, ok bool) {
	if code, ok := parseFlags(fs, args); !ok {
		return "", code, false
	}
	if fs.NArg() > 0 {
		id = fs.Arg(0)
		if code, ok := parseFlags(fs, fs.Args()[1:]); !ok {
			return "", code, false
		}
	}
	if id == "" || fs.NArg() > 0 {
		fs.Usage()
		return "", 2, false
	}
	return id, 0, true
}

// compareCmd exits 0 when the candidate is non-inferior with no improvement flagged, 1 on any
// regression, 3 on an improvement to review, and 2 on a usage error or a pair that cannot be
// compared: another role or slot, another case set, another k, or missing trials (design.md §7).
func compareCmd(args []string) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	baseID := fs.String("baseline", "", "baseline run id; the run's baselines/<role>[-no-flow][-<slot>].json when empty")
	margin := fs.Float64("margin", 5, "non-inferiority margin in score points")
	runsDir := fs.String("runs", "runs", "directory holding the run records")
	baselines := fs.String("baselines", "baselines", "directory holding baselines/<role>[-no-flow][-<slot>].json")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: flow-eval compare <candidate-run-id> [--baseline <run-id>] [--margin 5]")
		fs.PrintDefaults()
	}
	candID, code, ok := parseWithID(fs, args)
	if !ok {
		return code
	}
	if *margin < 0 {
		fs.Usage()
		return 2
	}
	cand, err := compare.Load(*runsDir, candID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval compare: %v\n", err)
		return 2
	}
	if *baseID == "" {
		if *baseID, err = run.BaselineID(*baselines, cand.Header.Role, cand.Header.Slot, cand.Header.NoFlow); err != nil {
			fmt.Fprintf(os.Stderr, "flow-eval compare: %v\n", err)
			return 2
		}
	}
	base, err := compare.Load(*runsDir, *baseID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval compare: %v\n", err)
		return 2
	}
	d, err := compare.Compare(cand, base, *margin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval compare: %v\n", err)
		return 2
	}
	path, err := compare.WriteReport(*runsDir, cand.ID, base.ID, d)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval compare: %v\n", err)
		return 2
	}
	for _, w := range d.Warnings() {
		fmt.Fprintln(os.Stderr, w)
	}
	fmt.Printf("%s\n%s\nreport: %s\n", d.Verdict, d.FlowEval, path)
	return d.Exit
}

// baselineCmd exits 0 when baselines/<role>[-no-flow][-<slot>].json now names the run, 2 on a usage error or a run
// that cannot be read. It then repaints the runs index and pages by the new baseline; a repaint
// failure is a warning on stderr and leaves the exit at 0, since the baseline is recorded.
func baselineCmd(args []string) int {
	if len(args) == 0 || args[0] != "set" {
		fmt.Fprintln(os.Stderr, "usage: flow-eval baseline set <run-id> [--runs runs] [--baselines baselines]")
		return 2
	}
	fs := flag.NewFlagSet("baseline set", flag.ContinueOnError)
	runsDir := fs.String("runs", "runs", "directory holding the run records")
	baselines := fs.String("baselines", "baselines", "directory holding baselines/<role>[-no-flow][-<slot>].json")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: flow-eval baseline set <run-id> [--runs runs] [--baselines baselines]")
		fs.PrintDefaults()
	}
	id, code, ok := parseWithID(fs, args[1:])
	if !ok {
		return code
	}
	key, err := run.SetBaseline(*runsDir, *baselines, id, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval baseline set: %v\n", err)
		return 2
	}
	fmt.Printf("%s baseline: %s\n", key, id)
	if err := errors.Join(run.Repaint(*runsDir, *baselines)); err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval baseline set: warning: baseline recorded, repaint failed: %v\n", err)
	}
	return 0
}

// regradeCmd exits 0 when every named run is re-graded, and 2 on a usage error, missing judge
// credentials, an invalid case, or a run that cannot be re-graded, naming it; the runs named before
// it stay re-graded.
func regradeCmd(args []string) int {
	fs := flag.NewFlagSet("regrade", flag.ContinueOnError)
	root := fs.String("root", "cases", "cases directory")
	runsDir := fs.String("runs", "runs", "directory holding the run records")
	baselines := fs.String("baselines", "baselines", "directory holding baselines/<role>[-no-flow][-<slot>].json, read to paint the index")
	pricingPath := fs.String("pricing", "pricing.json", "model prices")
	judgePath := fs.String("judge", "judge.json", "judge pin")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: flow-eval regrade <run-id>… [--root cases] [--runs runs] [--baselines baselines]")
		fs.PrintDefaults()
	}
	var ids []string
	for rest := args; ; {
		if code, ok := parseFlags(fs, rest); !ok {
			return code
		}
		if fs.NArg() == 0 {
			break
		}
		ids, rest = append(ids, fs.Arg(0)), fs.Args()[1:]
	}
	if len(ids) == 0 {
		fs.Usage()
		return 2
	}
	judgeCreds := _harnesses["claude-code"].creds
	if len(firstCredential(judgeCreds)) == 0 {
		fmt.Fprintf(os.Stderr, "flow-eval regrade: the review grader's judge needs one of %s set\n", strings.Join(judgeCreds, ", "))
		return 2
	}
	pricing, judge, sha, err := grading(*pricingPath, *judgePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval regrade: %v\n", err)
		return 2
	}
	loaded, invalid, err := cases.LoadAll(*root, "review")
	if err == nil && len(invalid) > 0 {
		err = fmt.Errorf("invalid cases: %w", errors.Join(invalid...))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval regrade: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	cfg := run.Config{
		Cases: loaded, RunsDir: *runsDir, BaselinesDir: *baselines, FlowEvalSHA: sha,
		Pricing: pricing, Judge: judge, JudgePin: judge.Pin, Warnings: os.Stderr,
	}
	for _, id := range ids {
		if err := run.Regrade(ctx, cfg, id); err != nil {
			fmt.Fprintf(os.Stderr, "flow-eval regrade: %v\n", err)
			return 2
		}
		fmt.Printf("regraded %s: %s/%s.jsonl, %s/%s.md, %s/%s.html\n", id, *runsDir, id, *runsDir, id, *runsDir, id)
	}
	return 0
}

func caseCmd(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "validate":
			return caseValidate(args[1:])
		case "add":
			return caseAdd(args[1:])
		}
	}
	fmt.Fprintln(os.Stderr, "usage: flow-eval case add|validate [flags]")
	return 2
}

const _caseAddUsage = `usage: flow-eval case add impl --repo <path> [--plan-repo <path>] --change <name> --task <n> [--root cases]
       flow-eval case add review --repo <path> [--plan-repo <path>] --change <name> [--root cases]
       flow-eval case add visual --id <id> --mockup <png> --capture <png> --verdict match|mismatch
                                 [--defect "<text>" [--region x,y,w,h]]... [--compose <path>] [--root cases]`

// caseAdd drafts a case under --root and exits 0 printing its directory, or 2 on a usage error or
// a case that cannot be mined. A --region belongs to the --defect before it.
func caseAdd(args []string) int {
	if len(args) == 0 || !slices.Contains([]string{"impl", "review", "visual"}, args[0]) {
		fmt.Fprintln(os.Stderr, _caseAddUsage)
		return 2
	}
	role := args[0]
	fs := flag.NewFlagSet("case add "+role, flag.ContinueOnError)
	root := fs.String("root", "cases", "cases directory the draft is written under")
	repo := fs.String("repo", "", "source checkout holding the change's commits (impl, review)")
	planRepo := fs.String("plan-repo", "", "checkout holding spectre/changes/archive/<change>, when not --repo (impl, review)")
	change := fs.String("change", "", "archived change name (impl, review)")
	task := fs.Int("task", 0, "task number whose implementer commit is the reference (impl)")
	draft := mine.VisualDraft{}
	fs.StringVar(&draft.ID, "id", "", "case id (visual)")
	fs.StringVar(&draft.Mockup, "mockup", "", "mockup PNG (visual)")
	fs.StringVar(&draft.Capture, "capture", "", "capture PNG (visual)")
	fs.StringVar(&draft.Verdict, "verdict", "", "match or mismatch (visual)")
	compose := "compose-mockup-frames.sh" // resolved through PATH when the home directory is unknown
	if home, err := os.UserHomeDir(); err == nil {
		compose = filepath.Join(home, ".claude/skills/flow/scripts/compose-mockup-frames.sh")
	}
	fs.StringVar(&draft.Compose, "compose", compose,
		"flow's compose-mockup-frames.sh, which crops a mockup board to its frame (visual)")
	fs.Func("defect", "a labelled defect of a mismatch; repeatable (visual)", func(s string) error {
		draft.Defects = append(draft.Defects, mine.Defect{Summary: s})
		return nil
	})
	fs.Func("region", "x,y,w,h box of the preceding --defect (visual)", func(s string) error {
		if len(draft.Defects) == 0 {
			return errors.New("a --region must follow its --defect")
		}
		r, err := parseRegion(s)
		draft.Defects[len(draft.Defects)-1].Region = r
		return err
	})
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), _caseAddUsage)
		fs.PrintDefaults()
	}
	if code, ok := parseFlags(fs, args[1:]); !ok {
		return code
	}
	var missing bool
	switch role {
	case "impl":
		missing = *repo == "" || *change == "" || *task < 1
	case "review":
		missing = *repo == "" || *change == ""
	case "visual":
		missing = draft.ID == "" || draft.Mockup == "" || draft.Capture == "" || draft.Verdict == ""
	}
	if missing || fs.NArg() > 0 {
		fs.Usage()
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	var dir string
	var err error
	switch role {
	case "impl":
		dir, err = mine.Impl(ctx, *root, *repo, cmp.Or(*planRepo, *repo), *change, *task)
	case "review":
		dir, err = mine.Review(ctx, *root, *repo, cmp.Or(*planRepo, *repo), *change)
	case "visual":
		dir, err = mine.Visual(ctx, *root, draft)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval case add %s: %v\n", role, err)
		return 2
	}
	fmt.Printf("draft %s: edit labels, then commit\n", dir)
	return 0
}

// parseRegion parses an x,y,w,h box of non-negative integers.
func parseRegion(s string) (*mine.Region, error) {
	var n [4]int
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("region %q is not x,y,w,h", s)
	}
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || v < 0 {
			return nil, fmt.Errorf("region %q is not x,y,w,h of non-negative integers", s)
		}
		n[i] = v
	}
	return &mine.Region{X: n[0], Y: n[1], W: n[2], H: n[3]}, nil
}

// caseValidate exits 0 when every case is valid, 1 listing each invalid case and why, 2 on a usage
// error or a root or role that cannot be read.
func caseValidate(args []string) int {
	fs := flag.NewFlagSet("case validate", flag.ContinueOnError)
	role := fs.String("role", "", "validate only this role's cases (impl, review, visual); all when empty")
	root := fs.String("root", "cases", "cases directory")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: flow-eval case validate [--role R] [--root cases]")
		fs.PrintDefaults()
	}
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return 2
	}
	loaded, invalid, err := cases.LoadAll(*root, *role)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval case validate: %v\n", err)
		return 2
	}
	for _, e := range invalid {
		fmt.Fprintf(os.Stderr, "invalid: %v\n", e)
	}
	if len(invalid) > 0 {
		return 1
	}
	hash, err := cases.SetHash(loaded)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval case validate: %v\n", err)
		return 2
	}
	fmt.Printf("%d cases valid, set %s\n", len(loaded), hash)
	return 0
}

func promptsCmd(args []string) int {
	if len(args) > 0 && args[0] == "sync" {
		return promptsSync(args[1:])
	}
	fmt.Fprintln(os.Stderr, "usage: flow-eval prompts sync --agents <path> --sha <sha> [--dir prompts]")
	return 2
}

// promptsSync exits 0 when every prompt was vendored, 2 on a usage error or a failed sync (an
// unknown sha, a strip heading upstream no longer has).
func promptsSync(args []string) int {
	fs := flag.NewFlagSet("prompts sync", flag.ContinueOnError)
	agents := fs.String("agents", "", "path to the agents repository")
	sha := fs.String("sha", "", "agents commit to vendor the prompts from")
	dir := fs.String("dir", "prompts", "prompts directory holding strip.txt")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: flow-eval prompts sync --agents <path> --sha <sha> [--dir prompts]")
		fs.PrintDefaults()
	}
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	if fs.NArg() > 0 || *agents == "" || *sha == "" {
		fs.Usage()
		return 2
	}
	if err := prompts.Sync(context.Background(), *dir, *agents, *sha); err != nil {
		fmt.Fprintf(os.Stderr, "flow-eval: %v\n", err)
		return 2
	}
	fmt.Printf("prompts vendored from %s\n", *sha)
	return 0
}

// parseFlags parses args into fs. Requested help prints the usage to stdout and returns 0; a bad
// flag prints the error and the usage to stderr and returns 2; ok is true only when parsing
// succeeded and the subcommand should run.
func parseFlags(fs *flag.FlagSet, args []string) (code int, ok bool) {
	usage := fs.Usage
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	err := fs.Parse(args)
	fs.Usage = usage
	switch {
	case errors.Is(err, flag.ErrHelp):
		fs.SetOutput(os.Stdout)
		fs.Usage()
		return 0, false
	case err != nil:
		fs.SetOutput(os.Stderr)
		fmt.Fprintln(os.Stderr, err)
		fs.Usage()
		return 2, false
	}
	fs.SetOutput(os.Stderr)
	return 0, true
}
