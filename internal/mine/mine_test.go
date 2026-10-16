package mine

import (
	json "encoding/json/v2"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/tweety53/flow-eval/internal/cases"
	"github.com/tweety53/flow-eval/internal/grade"
)

// _change is the archived gymie change the testdata/kan-757 artifacts are verbatim copies of.
const _change = "kan-757-offline-l5-be-per-item-if-match-on-group-session"

// _ledgerSHAs are the commits the verbatim ledger.md names, in fixture commit order: tasks 1–5,
// then panel-fix-1. newFixture replaces each with the fixture commit made in its place.
var _ledgerSHAs = []string{"05e022c3", "d8b3855f", "5a20e08f", "1d3d44b0", "80b40f62", "6ec0fe6a"}

const (
	_patchSvc = "src/app/workout/src/main/kotlin/com/gymie/workout/core/services/workout/impl/WorkoutFieldPatchServiceImpl.kt"
	_unitTest = "src/app/workout/src/test/kotlin/com/gymie/workout/core/services/workout/impl/" +
		"WorkoutFieldPatchServiceImplSetConflictTest.kt"
	_itTest  = "src/app/workout/src/test/kotlin/com/gymie/workout/infrastructure/persistence/WorkoutSetAuthorshipIntegrationTest.kt"
	_handler = "src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/" +
		"GroupSessionExceptionHandler.kt"
	_conflictTest = "src/app/groupsession/src/test/kotlin/com/gymie/groupsession/infrastructure/presentation/web/" +
		"GroupSessionFloorSetConflictTest.kt"
	// _leak is written into the archived plan only: the plan at the task's base never carries it.
	_leak = "LATER-CORRECTION-FROM-REVIEW-FIXES"
)

type fixture struct {
	repo    string
	base    string   // the parent of task 1's commit
	commits []string // full shas, aligned with _ledgerSHAs
}

// newFixture builds a gymie-shaped repository: a base commit, one commit per ledger dispatch that
// committed, and the change's archive holding the verbatim kan-757 artifacts with the ledger's shas
// rewritten to the fixture's. edits rewrites an artifact (by file name) before it is written.
func newFixture(t *testing.T, edits map[string]func(string) string) fixture {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "gymie")
	project, err := os.ReadFile("testdata/project.md")
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, repo, map[string]string{
		".flow/project.md":                      string(project),
		"src/app/workout/build.gradle.kts":      "dependencies {\n    testImplementation(\"org.testcontainers:postgresql\")\n}\n",
		"src/app/groupsession/build.gradle.kts": "dependencies {}\n",
		_patchSvc:                               "class WorkoutFieldPatchServiceImpl {\n}\n",
		_handler:                                "class GroupSessionExceptionHandler {\n}\n",
	})
	for _, name := range []string{"tasks.md", "proposal.md", "design.md"} {
		raw, err := os.ReadFile(filepath.Join("testdata/kan-757", name))
		if err != nil {
			t.Fatal(err)
		}
		writeFiles(t, filepath.Join(repo, "spectre/changes", _change), map[string]string{name: string(raw)})
	}
	git(t, repo, "init", "-q")
	commit := func(msg string) string {
		git(t, repo, "add", "-A")
		git(t, repo, "commit", "-q", "-m", msg)
		return strings.TrimSpace(git(t, repo, "rev-parse", "HEAD"))
	}
	f := fixture{repo: repo, base: commit("base")}
	for i, files := range []map[string]string{
		{
			_patchSvc: "class WorkoutFieldPatchServiceImpl {\n    fun patch() = Unit\n}\n",
			_unitTest: "class WorkoutFieldPatchServiceImplSetConflictTest {\n    @Test fun stale() = Unit\n}\n",
			_itTest:   "class WorkoutSetAuthorshipIntegrationTest {\n    @Test fun stamps() = Unit\n}\n",
		},
		{
			_handler:      "class GroupSessionExceptionHandler {\n    fun code() = Unit\n}\n",
			_conflictTest: "class GroupSessionFloorSetConflictTest {\n    @Test fun conflict() = Unit\n}\n",
			"spectre/changes/" + _change + "/tasks.md": readFile(t, "testdata/kan-757/tasks.md") + "\n<!-- ticked mid-change -->\n",
		},
		{_handler: "class GroupSessionExceptionHandler {\n    fun code() = Unit\n    fun setChanged() = Unit\n}\n"},
		{"src/app/workout/src/main/kotlin/com/gymie/workout/infrastructure/presentation/web/IdempotencyKeyFilter.kt": "class F\n"},
		{"spectre/specs/group-sessions.md": "# group sessions\n"},
		{_patchSvc: "class WorkoutFieldPatchServiceImpl {\n    fun patch() = if (changed) Unit else Unit\n}\n"},
	} {
		writeFiles(t, repo, files)
		f.commits = append(f.commits, commit("commit "+_ledgerSHAs[i]))
	}
	archive := filepath.Join(repo, "spectre/changes/archive", _change)
	for _, name := range []string{"ledger.md", "panel.md", "tasks.md", "proposal.md", "design.md"} {
		raw, err := os.ReadFile(filepath.Join("testdata/kan-757", name))
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		for i, sha := range _ledgerSHAs {
			body = strings.ReplaceAll(body, sha, f.commits[i][:8])
		}
		if name == "tasks.md" {
			body = strings.Replace(body, "- [x] 1. Set authorship stamps and the compare-and-set in workout\n",
				"- [x] 1. Set authorship stamps and the compare-and-set in workout\n\n  "+_leak+"\n", 1)
		}
		if name == "design.md" {
			body += "\n" + _leak + "\n"
		}
		if edit := edits[name]; edit != nil {
			body = edit(body)
		}
		writeFiles(t, archive, map[string]string{name: body})
	}
	return f
}

// replace returns an edit replacing old with new, failing the test when old is absent so an edit
// never silently stops applying.
func replace(t *testing.T, old, new string) func(string) string {
	t.Helper()
	return func(s string) string {
		if !strings.Contains(s, old) {
			t.Fatalf("fixture edit: %q not found", old)
		}
		return strings.Replace(s, old, new, 1)
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// gitAt runs git with its author and committer dates pinned.
func gitAt(t *testing.T, dir, author, committer string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_DATE="+author, "GIT_COMMITTER_DATE="+committer)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// loadOne loads root's only case of role through the real case loader, failing on any invalid case.
func loadOne(t *testing.T, root, role string) cases.Case {
	t.Helper()
	loaded, invalid, err := cases.LoadAll(root, role)
	if err != nil || len(invalid) > 0 || len(loaded) != 1 {
		t.Fatalf("LoadAll(%s): cases %v, invalid %v, err %v", role, loaded, invalid, err)
	}
	return loaded[0]
}

var _diffGit = regexp.MustCompile(`(?m)^diff --git a/(\S+) b/`)

func TestMineImplFromLedger(t *testing.T) {
	f := newFixture(t, nil)
	statusBefore := git(t, f.repo, "status", "--porcelain")
	root := t.TempDir()
	dir, err := Impl(t.Context(), root, f.repo, f.repo, _change, 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "impl", _change+"-task-1"); dir != want {
		t.Errorf("draft dir %q, want %q", dir, want)
	}
	c := loadOne(t, root, "impl")
	if c.Snapshot != f.base || c.Source.Repo != "gymie" || c.Source.Change != _change || !c.NeedsDocker {
		t.Errorf("case.json: snapshot %q (want task 1's parent %q), source %+v, needsDocker %v",
			c.Snapshot, f.base, c.Source, c.NeedsDocker)
	}
	wantVerify := []string{
		"./gradlew ktlintFormat",
		"./gradlew verifyChange",
		"./gradlew :workout:test --tests 'WorkoutFieldPatchServiceImplSetConflictTest' " +
			"--tests 'WorkoutSetAuthorshipIntegrationTest'",
		"./gradlew :workout:test",
	}
	if !slices.Equal(c.Verify, wantVerify) {
		t.Errorf("verify %q,\nwant %q", c.Verify, wantVerify)
	}
	if len(c.Files) != 9 || c.Files[0] != "src/app/src/main/resources/db/migration/V40__group_session_floor_conflicts.sql" ||
		c.Files[8] != _itTest {
		t.Errorf("files are not task 1's **Files:** field: %q", c.Files)
	}

	heldout := readFile(t, filepath.Join(dir, "heldout.patch"))
	var heldoutFiles []string
	for _, m := range _diffGit.FindAllStringSubmatch(heldout, -1) {
		heldoutFiles = append(heldoutFiles, m[1])
	}
	if want := []string{_unitTest, _itTest}; !slices.Equal(heldoutFiles, want) {
		t.Errorf("heldout.patch holds %q, want the reference commit's test files %q", heldoutFiles, want)
	}
	check := filepath.Join(t.TempDir(), "check")
	git(t, f.repo, "worktree", "add", "-q", "--detach", check, c.Snapshot)
	git(t, check, "apply", "--check", filepath.Join(dir, "heldout.patch"))

	if stat := readFile(t, filepath.Join(dir, "reference.stat")); !strings.Contains(stat, "WorkoutFieldPatchServiceImpl.kt") ||
		!strings.Contains(stat, "3 files changed") {
		t.Errorf("reference.stat is not the reference commit's diff stat:\n%s", stat)
	}
	task := readFile(t, filepath.Join(dir, "task.md"))
	if !strings.HasPrefix(task, "- [x] 1. Set authorship stamps") || strings.Contains(task, "- [x] 2. ") {
		t.Errorf("task.md is not task 1's section:\n%.300s", task)
	}
	ctxt := readFile(t, filepath.Join(dir, "context.md"))
	if !strings.Contains(ctxt, readFile(t, "testdata/kan-757/proposal.md")) ||
		!strings.Contains(ctxt, readFile(t, "testdata/kan-757/design.md")) {
		t.Error("context.md does not carry proposal.md and design.md")
	}
	if strings.Contains(task, _leak) || strings.Contains(ctxt, _leak) {
		t.Error("task.md or context.md carries the archived plan's later correction: the plan must be read at the task's base")
	}
	git(t, f.repo, "worktree", "remove", check)
	if after := git(t, f.repo, "status", "--porcelain"); after != statusBefore {
		t.Errorf("mining changed the source checkout: status %q, was %q", after, statusBefore)
	}

	t.Run("a checkout declaring no lint gets the test commands alone", func(t *testing.T) {
		f := newFixture(t, nil)
		writeFiles(t, f.repo, map[string]string{".flow/project.md": "# project\n"})
		git(t, f.repo, "commit", "-q", "-am", "no lint")
		sha := strings.TrimSpace(git(t, f.repo, "rev-parse", "HEAD"))
		verify, _, err := verifyCommands(t.Context(), f.repo, sha, []string{_unitTest})
		want := []string{"./gradlew :workout:test --tests 'WorkoutFieldPatchServiceImplSetConflictTest'", "./gradlew :workout:test"}
		if err != nil || !slices.Equal(verify, want) {
			t.Errorf("verify %q, %v; want %q", verify, err, want)
		}
	})
	t.Run("a renamed test file is held out as a deletion and an addition", func(t *testing.T) {
		f := newFixture(t, nil)
		renamed := strings.Replace(_unitTest, "SetConflictTest", "StaleSetTest", 1)
		git(t, f.repo, "mv", _unitTest, renamed)
		git(t, f.repo, "commit", "-q", "-m", "rename")
		ref := strings.TrimSpace(git(t, f.repo, "rev-parse", "HEAD"))
		ledger := filepath.Join(f.repo, "spectre/changes/archive", _change, "ledger.md")
		writeFiles(t, filepath.Dir(ledger), map[string]string{"ledger.md": readFile(t, ledger) +
			"\n## Dispatch 98\n\n- Task: 1\n- Role: implementer\n- Key: task-1-implementer-retry\n- Commit: " + ref[:8] + "\n"})
		dir, err := Impl(t.Context(), t.TempDir(), f.repo, f.repo, _change, 1)
		if err != nil {
			t.Fatal(err)
		}
		if patch := readFile(t, filepath.Join(dir, "heldout.patch")); strings.Contains(patch, "rename from") ||
			!strings.Contains(patch, "deleted file mode") || !strings.Contains(patch, "new file mode") {
			t.Errorf("heldout.patch records a rename, which the grader's reset cannot undo:\n%s", patch)
		}
	})
	t.Run("a cross-repository commit map yields the sha that is a commit in the code checkout", func(t *testing.T) {
		f := newFixture(t, nil)
		ref := f.commits[1]
		ledger := filepath.Join(f.repo, "spectre/changes/archive", _change, "ledger.md")
		writeFiles(t, filepath.Dir(ledger), map[string]string{"ledger.md": readFile(t, ledger) +
			"\n## Dispatch 97\n\n- Task: 1\n- Role: implementer\n- Key: task-1-implementer-retry\n- Commit: /wt/gymie=" +
			strings.Repeat("e", 40) + ",/wt/gymie-frontend=" + ref[:8] + "\n"})
		root := t.TempDir()
		if _, err := Impl(t.Context(), root, f.repo, f.repo, _change, 1); err != nil {
			t.Fatal(err)
		}
		if c := loadOne(t, root, "impl"); c.Snapshot != f.commits[0] {
			t.Errorf("snapshot %s, want the mapped commit's parent %s", c.Snapshot, f.commits[0])
		}
	})
	t.Run("a fix round's commit is never the reference", func(t *testing.T) {
		f := newFixture(t, nil)
		ledger := filepath.Join(f.repo, "spectre/changes/archive", _change, "ledger.md")
		writeFiles(t, filepath.Dir(ledger), map[string]string{"ledger.md": readFile(t, ledger) +
			"\n## Dispatch 99\n\n- Task: 1\n- Role: implementer\n- Key: task-1-implementer-fix-1\n- Commit: " + f.commits[5][:8] + "\n"})
		root := t.TempDir()
		if _, err := Impl(t.Context(), root, f.repo, f.repo, _change, 1); err != nil {
			t.Fatal(err)
		}
		if c := loadOne(t, root, "impl"); c.Snapshot != f.base {
			t.Errorf("snapshot %s, want task 1's own base %s, not the fix round's parent", c.Snapshot, f.base)
		}
	})
	t.Run("a change planned in another repository reads that repository's plan as of the snapshot", func(t *testing.T) {
		f := newFixture(t, nil)
		plans := filepath.Join(t.TempDir(), "gymie-plans")
		dir := filepath.Join(plans, "spectre/changes", _change)
		// Authored before and after the snapshot, both re-committed (as a rebase does) long after it.
		for i, date := range []string{"2001-01-01T00:00:00Z", "2098-01-01T00:00:00Z"} {
			mark := "  PLANNED-IN-THE-PLAN-REPOSITORY\n" // only this repository's plan carries it
			if i == 1 {
				mark += "  " + _leak + "\n"
			}
			tasks := strings.Replace(readFile(t, "testdata/kan-757/tasks.md"),
				"- [x] 1. Set authorship stamps and the compare-and-set in workout\n",
				"- [x] 1. Set authorship stamps and the compare-and-set in workout\n\n"+mark, 1)
			writeFiles(t, dir, map[string]string{"tasks.md": tasks,
				"proposal.md": readFile(t, "testdata/kan-757/proposal.md"), "design.md": readFile(t, "testdata/kan-757/design.md")})
			if i == 0 {
				git(t, plans, "init", "-q")
			}
			git(t, plans, "add", "-A")
			gitAt(t, plans, date, "2099-01-01T00:00:00Z", "commit", "-q", "-m", "plan")
		}
		if err := os.Rename(filepath.Join(f.repo, "spectre/changes/archive"), filepath.Join(plans, "spectre/changes/archive")); err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		draft, err := Impl(t.Context(), root, f.repo, plans, _change, 1)
		if err != nil {
			t.Fatal(err)
		}
		if task := readFile(t, filepath.Join(draft, "task.md")); !strings.Contains(task, "PLANNED-IN-THE-PLAN-REPOSITORY") || strings.Contains(task, _leak) {
			t.Errorf("task.md is not the plan as of the snapshot:\n%.200s", task)
		}
		if _, err := Review(t.Context(), root, f.repo, plans, _change); err != nil {
			t.Errorf("Review with the archive in another repository: %v", err)
		}
	})
	t.Run("a task the ledger has no implementer commit for is refused", func(t *testing.T) {
		if _, err := Impl(t.Context(), t.TempDir(), f.repo, f.repo, _change, 9); err == nil || !strings.Contains(err.Error(), "task 9") {
			t.Errorf("Impl(task 9) = %v, want an error naming task 9", err)
		}
	})
	t.Run("an existing draft is never overwritten", func(t *testing.T) {
		if _, err := Impl(t.Context(), root, f.repo, f.repo, _change, 1); err == nil {
			t.Error("Impl over an existing case directory succeeded")
		}
	})
}

// decodeReviewLabels decodes a review draft's labels.json, ignored[] included.
func decodeReviewLabels(t *testing.T, dir string) (labels grade.ReviewLabels, ignored []IgnoredLabel) {
	t.Helper()
	raw := readFile(t, filepath.Join(dir, "labels.json"))
	if err := json.Unmarshal([]byte(raw), &labels); err != nil {
		t.Fatal(err)
	}
	var withIgnored struct {
		Ignored []IgnoredLabel `json:"ignored"`
	}
	if err := json.Unmarshal([]byte(raw), &withIgnored); err != nil {
		t.Fatal(err)
	}
	return labels, withIgnored.Ignored
}

func ids[L any](labels []L, id func(L) string) []string {
	var out []string
	for _, l := range labels {
		out = append(out, id(l))
	}
	return out
}

func labelID(l grade.Label) string    { return l.ID }
func ignoredID(l IgnoredLabel) string { return l.ID }

func TestMineReviewLabelsFromPanel(t *testing.T) {
	const planFinding = "| F8 | primary | Important | spectre/changes/" + _change + "/tasks.md:63 | " +
		"Task 1's Files field omits WorkoutRepositoryAdapter.kt |   |\n"
	// Findings diff.patch cannot show — another repository's file, a file outside the range — and a
	// withdrawal that does not say the premise was false.
	const unseen = "| F9 | primary | Critical | gymie-frontend:shared/src/Sets.kt:3 | the client drops the 409 |   |\n" +
		"| F10 | primary | Important | src/app/workout/src/main/kotlin/Later.kt:5 | a fix-round file |   |\n" +
		"| F11 | principles | Important | " + _patchSvc + ":12 | duplicate of F1 |   |\n"
	f := newFixture(t, map[string]func(string) string{
		"panel.md": func(s string) string {
			s = replace(t, "finding-status: F6 withdrawn the omission assertion is gone — tests now assert the raw "+
				`"updatedAt":null body under decision unknown-actor-null`,
				"finding-status: F6 withdrawn premise false — the omission assertion does fail without the fix")(s)
			s = replace(t, "\nfindings-total: 7\n", planFinding+unseen+"\nfindings-total: 11\n")(s)
			return replace(t, "finding-status: F7 fixed\n", "finding-status: F7 fixed\nfinding-status: F8 fixed\n"+
				"finding-status: F9 fixed\nfinding-status: F10 fixed\nfinding-status: F11 withdrawn operator: duplicate of F1\n")(s)
		},
	})
	root := t.TempDir()
	dir, err := Review(t.Context(), root, f.repo, f.repo, _change)
	if err != nil {
		t.Fatal(err)
	}
	c := loadOne(t, root, "review")
	if head := f.commits[4]; c.Snapshot != head || dir != filepath.Join(root, "review", _change) {
		t.Errorf("draft %s with snapshot %q, want the last implementer commit before the panel-fix %q", dir, c.Snapshot, head)
	}
	want := git(t, f.repo, "diff", f.base, f.commits[4], "--", ".", ":(exclude)spectre/changes")
	if got := readFile(t, filepath.Join(dir, "diff.patch")); got != want || strings.Contains(got, "spectre/changes/") {
		t.Errorf("diff.patch is not the round-0 range base..head without the plan files:\n%s", got)
	}
	labels, ignored := decodeReviewLabels(t, dir)
	if got := ids(labels.MustFind, labelID); !slices.Equal(got, []string{"F1", "F4", "F5"}) {
		t.Errorf("mustFind %v, want the fixed Critical/Important code findings F1 F4 F5", got)
	}
	if got := ids(labels.KnownFalse, labelID); !slices.Equal(got, []string{"F2", "F6"}) {
		t.Errorf("knownFalse %v, want the false-premise withdrawals F2 F6", got)
	}
	if got := ids(ignored, ignoredID); !slices.Equal(got, []string{"F9", "F10", "F11"}) {
		t.Fatalf("ignored %v, want F9 F10 F11", got)
	}
	for i, reason := range []string{"in repository gymie-frontend, not gymie", "src/app/workout/src/main/kotlin/Later.kt is not in diff.patch", "operator: duplicate of F1"} {
		if ignored[i].Reason != reason {
			t.Errorf("ignored[%d].Reason = %q, want %q", i, ignored[i].Reason, reason)
		}
	}
	wantF1 := grade.Label{
		ID: "F1", File: _patchSvc, Line: 209, Severity: "Important", Slot: "primary",
		Summary: "An unchanged set PATCH is no longer a no-op: it writes, bumps the workout revision and re-attributes the set",
	}
	if !reflect.DeepEqual(labels.MustFind[0], wantF1) {
		t.Errorf("mustFind[0] = %+v,\nwant %+v", labels.MustFind[0], wantF1)
	}

	t.Run("a change with no implementer commit is refused", func(t *testing.T) {
		f := newFixture(t, map[string]func(string) string{
			"ledger.md": func(s string) string { return strings.ReplaceAll(s, "- Role: implementer", "- Role: verifier") },
		})
		if _, err := Review(t.Context(), t.TempDir(), f.repo, f.repo, _change); err == nil {
			t.Error("Review succeeded on a ledger with no implementer commit")
		}
	})
}

func TestLabelsCarryTheirSlot(t *testing.T) {
	// F2 is principles' duplicate of primary's F1; F3 cites a line of a changed file far outside
	// any hunk.
	const panel = `| ID | Slot | Severity | Location | Note | Lineage |
|---|---|---|---|---|---|
| F1 | primary | important | app/A.kt:10 | loadFailed drifts from the facts |   |
| F2 | principles | Important | app/A.kt:12 | loadFailed is a stored copy (Single Source of Truth) |   |
| F3 | mutation | Critical | app/A.kt:900 | the guard is untested |   |
| F4 | failure-modes | Important | app/A.kt:40 | the retry loops forever |   |
| F5 | mutation | Important | app/Gone.kt:1 | a fix-round file |   |

finding-status: F1 fixed
finding-status: F2 fixed
finding-status: F3 fixed
finding-status: F4 withdrawn premise false — the loop is bounded
finding-status: F5 fixed
`
	l := labelsFromPanel(panel, "gymie", []string{"app/A.kt"})
	type slotted struct{ id, slot string }
	var got []slotted
	for _, lb := range l.MustFind {
		got = append(got, slotted{lb.ID, lb.Slot})
	}
	want := []slotted{{"F1", "primary"}, {"F2", "principles"}, {"F3", "mutation"}}
	if !slices.Equal(got, want) {
		t.Errorf("mustFind %v, want %v: each slot's finding, a cross-slot duplicate and a site outside the hunks included", got, want)
	}
	if len(l.KnownFalse) != 1 || l.KnownFalse[0].Slot != "failure-modes" {
		t.Errorf("knownFalse %+v, want F4 carrying slot failure-modes", l.KnownFalse)
	}
	if len(l.Ignored) != 1 || l.Ignored[0].Slot != "mutation" {
		t.Errorf("ignored %+v, want F5 carrying slot mutation", l.Ignored)
	}
}

func TestMineReviewSupersededGoesToIgnored(t *testing.T) {
	// F6 keeps its verbatim reason (it names a decision); F2's is rewritten to name a supersession.
	f := newFixture(t, map[string]func(string) string{
		"panel.md": replace(t, "finding-status: F2 withdrawn premise false — ProblemDetail keeps null; contract restored "+
			"to explicit null under decision unknown-actor-null",
			"finding-status: F2 withdrawn premise superseded by R20's explicit-null wording"),
	})
	dir, err := Review(t.Context(), t.TempDir(), f.repo, f.repo, _change)
	if err != nil {
		t.Fatal(err)
	}
	labels, ignored := decodeReviewLabels(t, dir)
	if len(labels.KnownFalse) != 0 {
		t.Errorf("knownFalse %v, want none: both withdrawals were superseded, not false", ids(labels.KnownFalse, labelID))
	}
	if got := ids(ignored, ignoredID); !slices.Equal(got, []string{"F2", "F6"}) {
		t.Fatalf("ignored %v, want F2 F6", got)
	}
	if ignored[0].Reason != "premise superseded by R20's explicit-null wording" {
		t.Errorf("ignored[0].Reason = %q, want the withdrawal reason", ignored[0].Reason)
	}
}

func TestMineVisualCopiesImages(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "gymie-playwright")
	writeFiles(t, repo, map[string]string{
		"fidelity/home.png":                     "\x89PNG mockup",
		"fidelity/home.spec.ts-snapshots/1.png": "\x89PNG capture",
	})
	git(t, repo, "init", "-q")
	mockup, capture := filepath.Join(repo, "fidelity/home.png"), filepath.Join(repo, "fidelity/home.spec.ts-snapshots/1.png")
	outside := filepath.Join(t.TempDir(), "m.png")
	writeFiles(t, filepath.Dir(outside), map[string]string{"m.png": "\x89PNG"})
	root := t.TempDir()
	dir, err := Visual(t.Context(), root, VisualDraft{
		ID: "home-title", Mockup: mockup, Capture: capture, Verdict: "mismatch",
		Defects: []Defect{{Summary: "title sits 2px low", Region: &Region{X: 1, Y: 2, W: 30, H: 4}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := loadOne(t, root, "visual")
	if c.ID != "home-title" || c.Source.Repo != "gymie-playwright" || dir != filepath.Join(root, "visual", "home-title") {
		t.Errorf("case %+v in %s", c, dir)
	}
	for asset, src := range map[string]string{"mockup.png": mockup, "capture.png": capture} {
		if readFile(t, filepath.Join(dir, asset)) != readFile(t, src) {
			t.Errorf("%s is not a copy of %s", asset, src)
		}
	}
	var labels visualLabels
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(dir, "labels.json"))), &labels); err != nil {
		t.Fatal(err)
	}
	if labels.Verdict != "mismatch" || len(labels.Defects) != 1 || *labels.Defects[0].Region != (Region{X: 1, Y: 2, W: 30, H: 4}) {
		t.Errorf("labels.json = %+v", labels)
	}

	for _, tc := range []struct {
		name  string
		draft VisualDraft
	}{
		{"an unknown verdict", VisualDraft{ID: "v", Mockup: mockup, Capture: capture, Verdict: "maybe"}},
		{"a match listing a defect", VisualDraft{ID: "v", Mockup: mockup, Capture: capture, Verdict: "match",
			Defects: []Defect{{Summary: "x"}}}},
		{"a mismatch listing no defect", VisualDraft{ID: "v", Mockup: mockup, Capture: capture, Verdict: "mismatch"}},
		{"a defect without a summary", VisualDraft{ID: "v", Mockup: mockup, Capture: capture, Verdict: "mismatch",
			Defects: []Defect{{}}}},
		{"a missing capture", VisualDraft{ID: "v", Mockup: mockup, Capture: filepath.Join(repo, "absent.png"), Verdict: "match"}},
		{"a mockup outside a git checkout", VisualDraft{ID: "v", Mockup: outside, Capture: capture, Verdict: "match"}},
		{"an id that is a path", VisualDraft{ID: "../v", Mockup: mockup, Capture: capture, Verdict: "match"}},
		{"an existing case", VisualDraft{ID: "home-title", Mockup: mockup, Capture: capture, Verdict: "match"}},
	} {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			if _, err := Visual(t.Context(), root, tc.draft); err == nil {
				t.Errorf("Visual(%+v) succeeded", tc.draft)
			}
		})
	}
}

// board draws a gymie-shaped mockup board at scale 2, status 2, border 1: a 12×22 frame — a 1px
// border with a page-coloured top-left corner, a status band, and content — then a page margin and a
// caption band below it. content returns the content colour at its own x, y.
func board(content func(x, y int) color.RGBA) *image.RGBA {
	page, border := color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}
	img := image.NewRGBA(image.Rect(0, 0, 12, 34))
	for y := range 34 {
		for x := range 12 {
			c := page
			switch {
			case y < 22 && (y < 2 || y >= 20 || x < 2 || x >= 10):
				c = border
			case y < 6:
				c = color.RGBA{0, 0, 255, 255} // status band
			case y < 20:
				c = content(x-2, y-6)
			case y >= 26 && y < 30:
				c = color.RGBA{255, 0, 0, 255} // caption
			}
			img.Set(x, y, c)
		}
	}
	img.Set(0, 0, page)
	return img
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestMineVisualCropsTheMockupToItsFrame(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	compose := filepath.Join(home, ".claude/skills/flow/scripts/compose-mockup-frames.sh")
	if _, err := os.Stat(compose); err != nil {
		t.Skipf("flow's compose-mockup-frames.sh is not installed: %v", err)
	}
	content := func(x, y int) color.RGBA { return color.RGBA{uint8(x * 20), uint8(y * 10), 100, 255} }
	repo := filepath.Join(t.TempDir(), "gymie")
	writeFiles(t, repo, map[string]string{
		".flow/project.md": "| Key | Value |\n|-----|-------|\n| `mockup frame` | `scale=2 status=2 border=1` |\n",
	})
	if err := os.MkdirAll(filepath.Join(repo, "docs/mockups"), 0o755); err != nil {
		t.Fatal(err)
	}
	mockup := filepath.Join(repo, "docs/mockups/home.png")
	writePNG(t, mockup, board(content))
	git(t, repo, "init", "-q")
	capture := filepath.Join(t.TempDir(), "home-darwin.png")
	frame := image.NewRGBA(image.Rect(0, 0, 8, 14))
	for y := range 14 {
		for x := range 8 {
			frame.Set(x, y, content(x, y))
		}
	}
	writePNG(t, capture, frame)

	root := t.TempDir()
	dir, err := Visual(t.Context(), root, VisualDraft{ID: "home", Mockup: mockup, Capture: capture, Verdict: "match", Compose: compose})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "mockup.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Size() != image.Pt(8, 14) {
		t.Fatalf("mockup.png is %v, want the 8×14 frame without border, status band or caption", got.Bounds().Size())
	}
	for y := range 14 {
		for x := range 8 {
			if r, g, b, _ := got.At(x, y).RGBA(); (color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}) != content(x, y) {
				t.Fatalf("mockup.png at %d,%d is not the frame's content", x, y)
			}
		}
	}
	if c := loadOne(t, root, "visual"); c.Notes != "mockup docs/mockups/home.png, capture home-darwin.png" {
		t.Errorf("notes %q record host paths", c.Notes)
	}

	t.Run("a declared geometry with no compose script is refused", func(t *testing.T) {
		if _, err := Visual(t.Context(), t.TempDir(), VisualDraft{ID: "v", Mockup: mockup, Capture: capture, Verdict: "match"}); err == nil {
			t.Error("Visual copied an uncropped board")
		}
	})
}
