// Package grade turns a finished trial's output into scores against its case's ground truth
// (design.md §5), with no operator in the loop.
package grade

import (
	"archive/tar"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/tweety53/flow-eval/internal/cases"
	"github.com/tweety53/flow-eval/internal/metrics"
	"github.com/tweety53/flow-eval/internal/sandbox"
)

// Sandbox is the trial container the impl grader works in, after the agent has exited.
type Sandbox interface {
	// Exec runs cmd in /work and returns its exit code and output.
	Exec(ctx context.Context, cmd ...string) (sandbox.Result, error)
	// CopyIn extracts the tar archive at tarPath into dir.
	CopyIn(ctx context.Context, tarPath, dir string) error
}

var _ Sandbox = (*sandbox.Trial)(nil)

// Impl is an impl trial's grade.
type Impl struct {
	Passed bool
	// Reason says why a trial failed: heldout-conflict when the held-out tests do not apply over
	// the snapshot state of their paths, else the first verify command that exited non-zero. Empty
	// on a pass.
	Reason string
	Work   metrics.Work
	// Patch is the agent's change against the snapshot, and Log each verify command with its output
	// and exit, both kept as the trial's artifacts. Either is set as far as grading got.
	Patch, Log string
}

// _heldoutName is where the held-out patch lands: inside /work/.git, so no diff or verify command
// ever sees it as a work-tree file.
const _heldoutName = "flow-eval-heldout.patch"

// _junitScript prints every JUnit XML report Gradle left under a build/test-results directory,
// each followed by a NUL byte, which XML never contains.
const _junitScript = `find . -path '*/build/test-results/*' -name '*.xml' -type f ` +
	`-exec sh -c 'for f; do cat "$f"; printf "\0"; done' sh {} +`

// _testAnnotation is a JUnit @Test annotation, never @TestConfiguration or @Testcontainers: an
// extended regular expression both Go and grep read the same way.
const _testAnnotation = `@Test([^[:alnum:]_]|$)`

// _testDecl is an added line that declares a test: a JUnit @Test annotation or a Go test function.
var _testDecl = regexp.MustCompile(`(?m)^\+\s*(` + _testAnnotation + `|func Test)`)

// _git runs git with the agent-settable hooks that would run code under the grader switched off.
var _git = []string{"git", "-c", "core.fsmonitor=", "-c", "core.hooksPath=/dev/null"}

// _resetHeldout returns every path heldout.patch touches to its state at the snapshot ($1): checked
// out when the snapshot has it, removed when the patch adds it.
var _resetHeldout = `g="` + strings.Join(_git, " ") + `"
$g apply --numstat .git/` + _heldoutName + ` | cut -f3- | while IFS= read -r p; do
	if $g cat-file -e "$1:$p" 2>/dev/null; then $g checkout "$1" -- "$p"; else rm -f -- "$p"; fi
done`

// _clearReports removes the JUnit reports the agent's own test runs left, so only the verify
// commands' reports are read for pre-existing broken tests.
const _clearReports = `find . -path '*/build/test-results' -type d -prune -exec rm -rf {} +`

// _statLine is one file line of `git diff --stat`: the path, then its changed-line count.
var _statLine = regexp.MustCompile(`(?m)^\s*\S.*?\s+\|\s+(\d+)`)

// GradeImpl grades the agent's work in sb against c, and must run only after the agent's process
// has exited, since it is what brings the held-out tests into the container. It records the
// agent's diff (size against reference.stat, files outside c.Files, tests added), applies
// heldout.patch, then runs every c.Verify command in order, then the agent's own test classes by
// name in each module whose suite c.Verify drops (ownTestRuns); the trial passes when all exit 0.
// Failing JUnit tests whose source file the base snapshot already holds count as pre-existing
// broken. The agent's work is measured against the snapshot's root commit, so work it committed
// counts the same as work it left uncommitted. err is set when grading could not finish, the agent's
// tree being unusable or a verify command cut off by ctx included.
func GradeImpl(ctx context.Context, sb Sandbox, c cases.Case) (Impl, error) {
	var g Impl
	roots, err := run(ctx, sb, git("rev-list", "--max-parents=0", "HEAD")...)
	if err != nil {
		return g, err
	}
	base := strings.TrimSpace(roots)
	if strings.ContainsAny(base, "\n") || base == "" {
		return g, fmt.Errorf("snapshot: want one root commit, got %q", roots)
	}
	if _, err := run(ctx, sb, git("add", "-A")...); err != nil {
		return g, err
	}
	numstat, err := run(ctx, sb, git("diff", base, "--numstat", "-z", "--no-renames")...)
	if err != nil {
		return g, err
	}
	g.Work.DiffLines, g.Work.OutOfScopeFiles = diffStat(numstat, c.Files)
	g.Patch, err = run(ctx, sb, git("diff", base, "--no-color", "--no-renames", "--no-ext-diff")...)
	if err != nil {
		return g, err
	}
	g.Work.TestsAdded = len(_testDecl.FindAllString(g.Patch, -1))
	ref, err := os.ReadFile(filepath.Join(c.Dir, "reference.stat"))
	if err != nil {
		return g, err
	}
	if refLines := statLines(string(ref)); refLines > 0 {
		g.Work.OverbuildRatio = float64(g.Work.DiffLines) / float64(refLines)
	}

	if err := copyHeldout(ctx, sb, filepath.Join(c.Dir, "heldout.patch")); err != nil {
		return g, err
	}
	// The held-out tests replace whatever the agent left at their paths — task.md asks the agent to
	// write those very tests, so a plain apply would collide on every trial: each path the patch
	// touches goes back to its snapshot state first.
	if _, err := run(ctx, sb, "sh", "-c", _resetHeldout, "sh", base); err != nil {
		return g, err
	}
	res, err := sb.Exec(ctx, git("apply", ".git/"+_heldoutName)...)
	if err != nil {
		return g, fmt.Errorf("apply heldout.patch: %w", err)
	}
	if res.Exit != 0 {
		g.Reason = "heldout-conflict"
		return g, nil
	}

	if _, err := run(ctx, sb, "sh", "-c", _clearReports); err != nil {
		return g, err
	}
	own, err := ownTestRuns(ctx, sb, base, c.Verify)
	if err != nil {
		return g, err
	}
	// Every command runs even after one fails, so the module suite's reports exist to read.
	for _, cmd := range append(slices.Clone(c.Verify), own...) {
		res, err := sb.Exec(ctx, "sh", "-c", cmd)
		g.Log += fmt.Sprintf("$ %s\n%s%s[exit %d]\n", cmd, res.Stdout, res.Stderr, res.Exit)
		if err != nil {
			return g, fmt.Errorf("verify %q: %w", cmd, err)
		}
		if res.Exit != 0 && g.Reason == "" {
			g.Reason = fmt.Sprintf("verify: %s: exit %d", cmd, res.Exit)
		}
	}
	g.Passed = g.Reason == ""
	g.Work.PreexistingBroken, err = preexistingBroken(ctx, sb, base)
	return g, err
}

func git(args ...string) []string { return append(slices.Clone(_git), args...) }

// ownTestRuns returns a --tests run of the test classes the agent added or modified against base
// — files declaring @Test — in each module whose suite command (./gradlew <task>) ends no verify
// command, an environment prefix allowed: a module whose suite runs already runs them. The held-out patch's paths are left out: the case's own selectors run them.
func ownTestRuns(ctx context.Context, sb Sandbox, base string, verify []string) ([]string, error) {
	changed, err := run(ctx, sb, git("diff", base, "--name-only", "-z", "--no-renames", "--diff-filter=AM")...)
	if err != nil {
		return nil, err
	}
	heldout, err := run(ctx, sb, git("apply", "--numstat", "-z", ".git/"+_heldoutName)...)
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	for rec := range strings.SplitSeq(heldout, "\x00") {
		if fields := strings.SplitN(rec, "\t", 3); len(fields) == 3 {
			held[fields[2]] = true
		}
	}
	paths := slices.DeleteFunc(strings.Split(changed, "\x00"), func(p string) bool {
		return held[p] || !_sourceSet.MatchString(p)
	})
	if len(paths) == 0 {
		return nil, nil
	}
	// Gradle fails a --tests run that matches no test, so a helper or fixture in a test source set
	// must not become a selector: only a file declaring @Test does. No shell, and no device or FIFO
	// read: the paths are the agent's. Exit 2 is a path grep could not read — a dangling symlink the
	// agent left — which declares no test; every readable match is still printed.
	res, err := sb.Exec(ctx, append([]string{"grep", "-l", "-s", "--null", "-D", "skip", "-E", "-e", _testAnnotation, "--"}, paths...)...)
	if err == nil && res.Exit > 2 {
		err = fmt.Errorf("exit %d: %s", res.Exit, strings.TrimSpace(res.Stderr))
	}
	if err != nil {
		return nil, fmt.Errorf("grep @Test: %w", err)
	}
	var cmds []string
	for _, r := range TestRuns(strings.Split(res.Stdout, "\x00")) {
		suite := " ./gradlew " + r.Task
		if !slices.ContainsFunc(verify, func(v string) bool { return strings.HasSuffix(" "+v, suite) }) {
			cmds = append(cmds, r.Command())
		}
	}
	return cmds, nil
}

// run runs a grading step in sb that must succeed and returns its stdout.
func run(ctx context.Context, sb Sandbox, cmd ...string) (string, error) {
	res, err := sb.Exec(ctx, cmd...)
	if err == nil && res.Exit != 0 {
		err = fmt.Errorf("exit %d: %s", res.Exit, strings.TrimSpace(res.Stderr))
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.Join(cmd, " "), err)
	}
	return res.Stdout, nil
}

// diffStat sums the changed lines of a `git diff --numstat -z --no-renames` and lists, sorted, the
// changed paths that match none of the scope globs (path.Match syntax, plus a trailing `/**`
// for everything under a directory). A binary file counts no lines.
func diffStat(numstat string, scope []string) (lines int, outOfScope []string) {
	for rec := range strings.SplitSeq(numstat, "\x00") {
		fields := strings.SplitN(rec, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		for _, n := range fields[:2] {
			v, _ := strconv.Atoi(n) // "-" for a binary file
			lines += v
		}
		inScope := slices.ContainsFunc(scope, func(glob string) bool {
			if dir, ok := strings.CutSuffix(glob, "/**"); ok {
				return strings.HasPrefix(fields[2], dir+"/")
			}
			ok, _ := path.Match(glob, fields[2]) // a malformed glob matches nothing
			return ok
		})
		if !inScope {
			outOfScope = append(outOfScope, fields[2])
		}
	}
	slices.Sort(outOfScope)
	return lines, outOfScope
}

// statLines sums the per-file changed-line counts of a `git diff --stat` listing.
func statLines(stat string) int {
	total := 0
	for _, m := range _statLine.FindAllStringSubmatch(stat, -1) {
		n, _ := strconv.Atoi(m[1]) // the pattern admits digits only
		total += n
	}
	return total
}

// copyHeldout copies the patch file at src into the container's /work/.git as _heldoutName.
func copyHeldout(ctx context.Context, sb Sandbox, src string) (err error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "flow-eval-heldout-*.tar")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.Remove(f.Name())) }()
	tw := tar.NewWriter(f)
	hdr := &tar.Header{Name: _heldoutName, Mode: 0o644, Size: int64(len(data)), Uname: "root", Gname: "root"}
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.Join(err, f.Close())
	}
	if _, err := tw.Write(data); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := errors.Join(tw.Close(), f.Close()); err != nil {
		return err
	}
	return sb.CopyIn(ctx, f.Name(), "/work/.git")
}

// junitCase is one <testcase> of a JUnit XML report.
type junitCase struct {
	Class   string    `xml:"classname,attr"`
	Name    string    `xml:"name,attr"`
	Failure *struct{} `xml:"failure"`
	Error   *struct{} `xml:"error"`
}

// preexistingBroken counts the distinct failing tests in the work tree's JUnit reports whose class
// has a source file in the base commit. ponytail: the source file is found by its class's package
// path and outer class name, so a Kotlin test class in a file named otherwise is missed; read a
// report's file attribute if a test runner ever writes one.
func preexistingBroken(ctx context.Context, sb Sandbox, base string) (int, error) {
	reports, err := run(ctx, sb, "sh", "-c", _junitScript)
	if err != nil {
		return 0, err
	}
	tree, err := run(ctx, sb, git("ls-tree", "-r", "--name-only", "-z", base)...)
	if err != nil {
		return 0, err
	}
	baseFiles := strings.Split(tree, "\x00")
	broken := map[string]bool{}
	for report := range strings.SplitSeq(reports, "\x00") {
		failing, err := failingCases(report)
		if err != nil {
			return 0, err
		}
		for _, tc := range failing {
			outer, _, _ := strings.Cut(tc.Class, "$")
			src := "/" + strings.ReplaceAll(outer, ".", "/") + "."
			if slices.ContainsFunc(baseFiles, func(p string) bool { return strings.Contains("/"+p, src) }) {
				broken[tc.Class+"#"+tc.Name] = true
			}
		}
	}
	return len(broken), nil
}

// failingCases returns the testcases of one JUnit XML report that carry a failure or an error.
func failingCases(report string) ([]junitCase, error) {
	var failing []junitCase
	dec := xml.NewDecoder(strings.NewReader(report))
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return failing, nil
		}
		if err != nil {
			return nil, fmt.Errorf("junit report: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "testcase" {
			continue
		}
		var tc junitCase
		if err := dec.DecodeElement(&tc, &start); err != nil {
			return nil, fmt.Errorf("junit report: %w", err)
		}
		if tc.Failure != nil || tc.Error != nil {
			failing = append(failing, tc)
		}
	}
}
