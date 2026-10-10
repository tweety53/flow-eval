package grade

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tweety53/flow-eval/internal/cases"
	"github.com/tweety53/flow-eval/internal/sandbox"
)

// hostTrial stands in for the trial container with a real git repository on the host: Exec runs
// the command for real in dir, as the container runs it in /work, and CopyIn extracts the tar with
// the host's tar into dir's matching path. Only the docker transport is replaced.
type hostTrial struct{ dir string }

func (h hostTrial) Exec(ctx context.Context, cmd ...string) (sandbox.Result, error) {
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Dir = h.dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	var stdout, stderr strings.Builder
	c.Stdout, c.Stderr = &stdout, &stderr
	err := c.Run()
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		err = nil
	}
	return sandbox.Result{Exit: c.ProcessState.ExitCode(), Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func (h hostTrial) CopyIn(ctx context.Context, tarPath, dir string) error {
	rel, ok := strings.CutPrefix(dir, "/work")
	if !ok {
		return &os.PathError{Op: "copyin", Path: dir, Err: os.ErrInvalid}
	}
	return exec.CommandContext(ctx, "tar", "-xf", tarPath, "-C", filepath.Join(h.dir, rel)).Run()
}

const _heldout = `diff --git a/src/app/ExampleTest.kt b/src/app/ExampleTest.kt
new file mode 100644
--- /dev/null
+++ b/src/app/ExampleTest.kt
@@ -0,0 +1,2 @@
+class ExampleTest {
+}
`

// newImplTrial commits base as the snapshot's one "base" commit, then writes agent over it as the
// agent's uncommitted work. It returns the trial and an impl case whose verify commands are verify.
func newImplTrial(t *testing.T, base, agent map[string]string, verify ...string) (hostTrial, cases.Case) {
	t.Helper()
	work := t.TempDir()
	writeFiles(t, work, base)
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "base"},
	} {
		c := exec.Command("git", args...)
		c.Dir = work
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	writeFiles(t, work, agent)
	caseDir := t.TempDir()
	writeFiles(t, caseDir, map[string]string{
		"heldout.patch":  _heldout,
		"reference.stat": " src/app/Example.kt     | 3 ++-\n src/app/Other.kt       | 1 +\n 2 files changed, 3 insertions(+), 1 deletion(-)\n",
	})
	c := cases.Case{
		ID: "i1", Role: "impl", Dir: caseDir, Verify: verify,
		Files: []string{"src/app/Example.kt", "src/app/*Test.kt", "src/app/fixtures/**"},
	}
	return hostTrial{dir: work}, c
}

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

var _base = map[string]string{"src/app/Example.kt": "class Example\n", "README.md": "readme\n"}

func TestImplPassRequiresAllCommands(t *testing.T) {
	tests := []struct {
		name       string
		agent      map[string]string
		verify     []string
		wantPassed bool
		wantReason string
	}{
		{
			name:       "every verify command exits 0 after the held-out tests apply",
			agent:      map[string]string{"src/app/Example.kt": "class Example { fun x() = 1 }\n"},
			verify:     []string{"test -f src/app/ExampleTest.kt", "true"},
			wantPassed: true,
		},
		{
			name:       "one failing verify command fails the trial",
			agent:      map[string]string{"src/app/Example.kt": "class Example { fun x() = 1 }\n"},
			verify:     []string{"true", "exit 3", "true"},
			wantReason: "verify: exit 3: exit 3",
		},
		{
			name:       "the held-out tests replace the agent's own test file at the same path",
			agent:      map[string]string{"src/app/ExampleTest.kt": "class ExampleTest(agent)\n"},
			verify:     []string{"grep -qx 'class ExampleTest {' src/app/ExampleTest.kt", "! grep -q agent src/app/ExampleTest.kt"},
			wantPassed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb, c := newImplTrial(t, _base, tt.agent, tt.verify...)
			got, err := GradeImpl(t.Context(), sb, c)
			if err != nil {
				t.Fatal(err)
			}
			if got.Passed != tt.wantPassed || got.Reason != tt.wantReason {
				t.Errorf("GradeImpl = passed %v, reason %q; want passed %v, reason %q",
					got.Passed, got.Reason, tt.wantPassed, tt.wantReason)
			}
			if _, err := os.Stat(filepath.Join(sb.dir, "verified")); err == nil && tt.wantReason == "heldout-conflict" {
				t.Error("verify ran after the held-out patch failed to apply")
			}
		})
	}
}

func TestImplOutOfScopeFiles(t *testing.T) {
	sb, c := newImplTrial(t, _base, map[string]string{
		"src/app/Example.kt":        "class Example { fun x() = 1 }\n", // declared
		"src/app/HelperTest.kt":     "class HelperTest\n",              // matches the src/app/*Test.kt glob
		"src/app/fixtures/a/b.json": "{}\n",                            // under src/app/fixtures/**
		"src/app/Extra.kt":          "class Extra\n",                   // new, undeclared
		"README.md":                 "changed\n",                       // modified, undeclared
		"src/app/build/out.class":   "",                                // ignored below, never counted
		".gitignore":                "build/\n",
	}, "true")
	got, err := GradeImpl(t.Context(), sb, c)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".gitignore", "README.md", "src/app/Extra.kt"}
	if !slices.Equal(got.Work.OutOfScopeFiles, want) {
		t.Errorf("OutOfScopeFiles = %q, want %q", got.Work.OutOfScopeFiles, want)
	}
}

func TestImplOverbuildRatio(t *testing.T) {
	// reference.stat changes 3+1 = 4 lines; the agent changes 8: Example.kt 1 deleted + 3 added,
	// and a new 4-line test file holding two tests.
	sb, c := newImplTrial(t, _base, map[string]string{
		"src/app/Example.kt":  "class Example {\n  fun x() = 1\n}\n",
		"src/app/MineTest.kt": "class MineTest {\n  @Test fun a() {}\n  @Test\n  fun b() {}\n",
	}, "true")
	got, err := GradeImpl(t.Context(), sb, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Work.DiffLines != 8 || got.Work.OverbuildRatio != 2 || got.Work.TestsAdded != 2 {
		t.Errorf("Work = diff %d, ratio %v, tests added %d; want 8, 2, 2",
			got.Work.DiffLines, got.Work.OverbuildRatio, got.Work.TestsAdded)
	}
}

func TestImplPreexistingBroken(t *testing.T) {
	// The verify command leaves Gradle-shaped JUnit XML behind: two failing tests whose source is in
	// the base snapshot (one in a nested class), one failing held-out test the base does not have,
	// and one passing base test.
	junit := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="com.ex.OldTest" tests="2" failures="1" errors="0">
  <testcase name="breaks" classname="com.ex.OldTest"><failure message="boom">trace</failure></testcase>
  <testcase name="holds" classname="com.ex.OldTest"/>
</testsuite>`
	nested := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="com.ex.OldTest$Inner" tests="1" failures="0" errors="1">
  <testcase name="errs" classname="com.ex.OldTest$Inner"><error message="npe"/></testcase>
</testsuite>`
	heldout := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="com.ex.ExampleTest" tests="1" failures="1" errors="0">
  <testcase name="new" classname="com.ex.ExampleTest"><failure message="x"/></testcase>
</testsuite>`
	base := map[string]string{
		"src/app/Example.kt":                    "class Example\n",
		"app/src/test/kotlin/com/ex/OldTest.kt": "class OldTest\n",
	}
	writeXML := "mkdir -p app/build/test-results/test && cd app/build/test-results/test && " +
		"printf '%s' '" + junit + "' > TEST-com.ex.OldTest.xml && " +
		"printf '%s' '" + nested + "' > 'TEST-com.ex.OldTest$Inner.xml' && " +
		"printf '%s' '" + heldout + "' > TEST-com.ex.ExampleTest.xml && exit 1"
	sb, c := newImplTrial(t, base, nil, writeXML)
	got, err := GradeImpl(t.Context(), sb, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Work.PreexistingBroken != 2 {
		t.Errorf("PreexistingBroken = %d, want 2", got.Work.PreexistingBroken)
	}
}

func TestImplCommittedWorkIsGraded(t *testing.T) {
	sb, c := newImplTrial(t, _base, map[string]string{
		"src/app/Example.kt":  "class Example {\n  fun x() = 1\n}\n",
		"src/app/MineTest.kt": "class MineTest {\n  @Test fun a() {}\n  @Test\n  fun b() {}\n",
	}, "true")
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.name=agent", "-c", "user.email=a@a", "commit", "-q", "-m", "agent work"},
	} {
		if res, err := sb.Exec(t.Context(), append([]string{"git"}, args...)...); err != nil || res.Exit != 0 {
			t.Fatalf("git %v: %v %s", args, err, res.Stderr)
		}
	}
	got, err := GradeImpl(t.Context(), sb, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Work.DiffLines != 8 || got.Work.TestsAdded != 2 || !got.Passed {
		t.Errorf("committed work graded as diff %d, tests added %d, passed %v; want 8, 2, true",
			got.Work.DiffLines, got.Work.TestsAdded, got.Passed)
	}
}

func TestImplIgnoresTheAgentsOwnTestReports(t *testing.T) {
	stale := `<testsuite name="com.ex.OldTest"><testcase name="breaks" classname="com.ex.OldTest"><failure/></testcase></testsuite>`
	base := map[string]string{"src/app/Example.kt": "class Example\n", "app/src/test/kotlin/com/ex/OldTest.kt": "class OldTest\n"}
	agent := map[string]string{"app/build/test-results/test/TEST-com.ex.OldTest.xml": stale}
	sb, c := newImplTrial(t, base, agent, "true")
	got, err := GradeImpl(t.Context(), sb, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Work.PreexistingBroken != 0 {
		t.Errorf("PreexistingBroken = %d from a report the agent left, want 0", got.Work.PreexistingBroken)
	}
}

func TestImplResetOfHeldoutPathsRunsNoAgentHook(t *testing.T) {
	base := map[string]string{"src/app/Example.kt": "class Example\n", "src/app/ExampleTest.kt": "class ExampleTest\n"}
	agent := map[string]string{
		"src/app/ExampleTest.kt":   "class ExampleTest { /* the agent's */ }\n",
		".git/hooks/post-checkout": "#!/bin/sh\ntouch hooked\n",
	}
	sb, c := newImplTrial(t, base, agent, "true")
	if err := os.Chmod(filepath.Join(sb.dir, ".git/hooks/post-checkout"), 0o755); err != nil {
		t.Fatal(err)
	}
	heldout := "diff --git a/src/app/ExampleTest.kt b/src/app/ExampleTest.kt\n" +
		"--- a/src/app/ExampleTest.kt\n+++ b/src/app/ExampleTest.kt\n" +
		"@@ -1 +1,2 @@\n class ExampleTest\n+// held out\n"
	writeFiles(t, c.Dir, map[string]string{"heldout.patch": heldout})
	got, err := GradeImpl(t.Context(), sb, c)
	if err != nil || got.Reason != "" {
		t.Fatalf("GradeImpl = reason %q, err %v; want the held-out patch applied", got.Reason, err)
	}
	if _, err := os.Stat(filepath.Join(sb.dir, "hooked")); err == nil {
		t.Error("the agent's post-checkout hook ran while the grader reset a held-out path")
	}
}

// gradleTrial is a hostTrial whose ./gradlew commands, environment prefix included, are recorded
// instead of run: the command named pass exits 0, every other one exits 1.
type gradleTrial struct {
	hostTrial
	pass string
	ran  *[]string
}

func (g gradleTrial) Exec(ctx context.Context, cmd ...string) (sandbox.Result, error) {
	if len(cmd) == 3 && cmd[0] == "sh" && strings.Contains(cmd[2], "./gradlew ") {
		*g.ran = append(*g.ran, cmd[2])
		if cmd[2] == g.pass {
			return sandbox.Result{}, nil
		}
		return sandbox.Result{Exit: 1, Stdout: "FAILED\n"}, nil
	}
	return g.hostTrial.Exec(ctx, cmd...)
}

// _testClass is a test source file declaring a test.
const _testClass = "class T {\n    @Test fun t() {}\n}\n"

func TestGradeImplRunsTheAgentsOwnTestClasses(t *testing.T) {
	const held = "shared/src/desktopTest/kotlin/HeldTest.kt"
	base := map[string]string{"shared/src/desktopTest/kotlin/OldTest.kt": "class OldTest\n"}
	agent := map[string]string{
		"shared/src/commonTest/kotlin/a/NewTest.kt": _testClass, // suite dropped: run by name
		"app/src/test/kotlin/AppTest.kt":            _testClass, // suite in verify: run there
		held:                                        "class HeldTest(agent)\n",
		"shared/src/main/kotlin/Shared.kt":          "class Shared\n", // not a test
	}
	sb, c := newImplTrial(t, base, agent, "./gradlew :app:test")
	if err := os.Remove(filepath.Join(sb.dir, "shared/src/desktopTest/kotlin/OldTest.kt")); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, c.Dir, map[string]string{"heldout.patch": "diff --git a/" + held + " b/" + held + `
new file mode 100644
--- /dev/null
+++ b/` + held + `
@@ -0,0 +1 @@
+@Test class HeldTest
`})
	var ran []string
	got, err := GradeImpl(t.Context(), gradleTrial{hostTrial: sb, pass: "./gradlew :app:test", ran: &ran}, c)
	if err != nil {
		t.Fatal(err)
	}
	own := "./gradlew :shared:desktopTest --tests 'NewTest'"
	if want := []string{"./gradlew :app:test", own}; !slices.Equal(ran, want) {
		t.Errorf("ran %q, want %q", ran, want)
	}
	if want := "verify: " + own + ": exit 1"; got.Passed || got.Reason != want {
		t.Errorf("GradeImpl = passed %v, reason %q; want failed, reason %q", got.Passed, got.Reason, want)
	}
	if !strings.Contains(got.Log, "$ "+own+"\nFAILED\n[exit 1]\n") {
		t.Errorf("log lacks the agent's own test run:\n%s", got.Log)
	}
}

func TestGradeImplOwnTestRunsSkip(t *testing.T) {
	tests := []struct {
		name   string
		agent  map[string]string
		link   string // a dangling symlink the agent leaves
		verify string
		want   []string
	}{
		{
			name: "a test-source file declaring no test",
			agent: map[string]string{
				"shared/src/commonTest/kotlin/FakeApi.kt": "class FakeApi\n",
				"shared/src/commonTest/kotlin/NewTest.kt": _testClass,
			},
			verify: "./gradlew ktlintCheck",
			want:   []string{"./gradlew ktlintCheck", "./gradlew :shared:desktopTest --tests 'NewTest'"},
		},
		{
			name:   "a module whose suite verify runs behind an environment prefix",
			agent:  map[string]string{"src/gateway/src/test/kotlin/NewGatewayTest.kt": _testClass},
			verify: "MANAGEMENT_HEALTH_REDIS_ENABLED=false ./gradlew :gateway:test",
			want:   []string{"MANAGEMENT_HEALTH_REDIS_ENABLED=false ./gradlew :gateway:test"},
		},
		{
			name:   "a module directory name that is no Gradle project name",
			agent:  map[string]string{"x$(touch PWNED)y/src/test/kotlin/ATest.kt": _testClass},
			verify: "./gradlew ktlintCheck",
			want:   []string{"./gradlew ktlintCheck"},
		},
		{
			name: "a test-source file whose only annotation merely starts with @Test",
			agent: map[string]string{
				"src/app/src/test/kotlin/AuthTestConfig.kt": "@TestConfiguration\nclass AuthTestConfig\n",
			},
			verify: "./gradlew ktlintCheck",
			want:   []string{"./gradlew ktlintCheck"},
		},
		{
			name:   "an unreadable test-source path beside a test",
			agent:  map[string]string{"shared/src/commonTest/kotlin/NewTest.kt": _testClass},
			link:   "shared/src/commonTest/kotlin/Gone.kt",
			verify: "./gradlew ktlintCheck",
			want:   []string{"./gradlew ktlintCheck", "./gradlew :shared:desktopTest --tests 'NewTest'"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb, c := newImplTrial(t, map[string]string{"README.md": "base\n"}, tt.agent, tt.verify)
			if tt.link != "" {
				if err := os.Symlink("missing", filepath.Join(sb.dir, tt.link)); err != nil {
					t.Fatal(err)
				}
			}
			var ran []string
			if _, err := GradeImpl(t.Context(), gradleTrial{hostTrial: sb, pass: tt.verify, ran: &ran}, c); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ran, tt.want) {
				t.Errorf("ran %q, want %q", ran, tt.want)
			}
		})
	}
}
