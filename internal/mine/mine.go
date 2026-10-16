// Package mine drafts eval cases from flow history in a source checkout (design.md §3): an impl
// case from one task's implementer commit, a review case from a change's round-0 range and panel,
// and a visual case from a mockup and capture pair. A miner only reads the source checkout, save
// fetching a change's pre-reshape ref from origin when the checkout lacks it; the operator edits the
// draft's labels, and the commit review is the label approval.
package mine

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tweety53/flow-eval/internal/cases"
	"github.com/tweety53/flow-eval/internal/grade"
)

// _archive is where a change's flow artifacts live in its source checkout.
const _archive = "spectre/changes/archive"

// IgnoredLabel is a withdrawn finding whose reason names a later decision or supersession rather
// than a false premise: neither must-find nor known-false until the operator moves it.
type IgnoredLabel struct {
	grade.Label
	Reason string `json:"reason"`
}

// reviewLabels is a review draft's labels.json: the grader's labels plus ignored[].
type reviewLabels struct {
	MustFind   []grade.Label  `json:"mustFind"`
	KnownFalse []grade.Label  `json:"knownFalse"`
	Ignored    []IgnoredLabel `json:"ignored"`
}

// Region is a defect's x,y,w,h box in capture pixels.
type Region struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Defect is one labelled departure of a mismatch case.
type Defect struct {
	Summary string  `json:"summary"`
	Region  *Region `json:"region,omitempty"`
}

// visualLabels is a visual draft's labels.json.
type visualLabels struct {
	Verdict string   `json:"verdict"`
	Defects []Defect `json:"defects"`
}

// VisualDraft is what a visual case is mined from: the case id, the mockup and capture PNGs, the
// labelled verdict with its defects, and flow's compose-mockup-frames.sh, which crops a mockup
// board down to its frame.
type VisualDraft struct {
	ID      string
	Mockup  string
	Capture string
	Verdict string // match or mismatch
	Defects []Defect
	Compose string
}

// Impl drafts root/impl/<change>-task-<n>/ from task n of change, whose commits are in the
// checkout repo and whose archive is in planRepo (repo itself for a single-repository change). The
// reference is the oldest commit carrying a `Task-Id: <n>` trailer reachable from the
// change's pre-reshape ref (see prereshape) and from no branch — integrate squashes the task commits, and a grouped
// implementer dispatch is one ledger row against the group's last commit — else, with no such
// commit, that of the ledger's last implementer dispatch for task n that is not a fix round. Its
// parent is the snapshot — a fix round's parent already holds the answer. It returns the draft
// directory, and refuses to overwrite an existing one.
func Impl(ctx context.Context, root, repo, planRepo, change string, n int) (string, error) {
	art := filepath.Join(planRepo, _archive, change)
	pin, fetchErr := prereshape(ctx, repo, change)
	if planRepo != repo {
		_, _ = prereshape(ctx, planRepo, change) // planAt reads the plan's pre-reshape history too
	}
	ledger, err := readLedger(ctx, art, repo)
	if err != nil {
		return "", err
	}
	var ref string
	for _, d := range ledger {
		if d.role == "implementer" && d.task == strconv.Itoa(n) && d.commit != "" && !strings.Contains(d.key, "-fix-") {
			ref = d.commit
		}
	}
	if pin != "" {
		found, err := taskCommits(ctx, repo, pin, strconv.Itoa(n))
		if err != nil {
			return "", err
		}
		if len(found) > 0 {
			ref = found[0]
		}
	}
	if ref == "" {
		return "", fmt.Errorf("%s: no implementer commit for task %d in ledger.md", change, n)
	}
	if ref, err = gitOut(ctx, repo, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
		return "", withFetch(err, fetchErr)
	}
	base, err := gitOut(ctx, repo, "rev-parse", "--verify", ref+"^")
	if err != nil {
		return "", err
	}

	// The plan as it stood at base, never the archived one: the archive carries later Corrections and
	// decisions written from review fixes landed after this task — the answer. In another repository
	// that is its last commit to the change's plan authored no later than the reference commit — the
	// plan the implementer worked from; base predates the change and can predate its plan.
	at := base
	if planRepo != repo {
		if at, err = planAt(ctx, repo, ref, planRepo, change); err != nil {
			return "", err
		}
	}
	plan := func(name string) (string, error) {
		return gitOut(ctx, planRepo, "show", at+":spectre/changes/"+change+"/"+name)
	}
	tasks, err := plan("tasks.md")
	if err != nil {
		return "", err
	}
	task, files, err := taskSection(tasks, n)
	if err != nil {
		return "", fmt.Errorf("%s: %w", change, err)
	}
	var background []byte
	for _, name := range []string{"proposal.md", "design.md"} {
		raw, err := plan(name)
		if err != nil {
			return "", err
		}
		background = fmt.Appendf(background, "# %s\n\n%s\n", name, raw)
	}

	changed, err := gitOut(ctx, repo, "diff", "--name-only", "--no-renames", base, ref)
	if err != nil {
		return "", err
	}
	heldout := slices.DeleteFunc(strings.Split(changed, "\n"), func(p string) bool { return !isTest(p) })
	if len(heldout) == 0 {
		return "", fmt.Errorf("%s: task %d's commit %s changes no test file to hold out", change, n, ref)
	}
	// --no-renames: the grader resets each path the patch names, and a rename names only its target.
	patch, err := gitOut(ctx, repo, append([]string{"diff", "--binary", "--no-renames", base, ref, "--"}, heldout...)...)
	if err != nil {
		return "", err
	}
	stat, err := gitOut(ctx, repo, "diff", "--stat", base, ref)
	if err != nil {
		return "", err
	}
	verify, needsDocker, err := verifyCommands(ctx, repo, base, heldout)
	if err != nil {
		return "", err
	}
	subject, err := gitOut(ctx, repo, "log", "-1", "--format=%s", ref)
	if err != nil {
		return "", err
	}

	c := cases.Case{
		ID: fmt.Sprintf("%s-task-%d", change, n), Role: "impl",
		Source:   cases.Source{Repo: filepath.Base(repo), Change: change},
		Snapshot: base, Notes: fmt.Sprintf("task %d: %s (reference %s)", n, subject, ref),
		NeedsDocker: needsDocker, Verify: verify, Files: files,
	}
	return write(root, c, map[string][]byte{
		"task.md":        []byte(task),
		"context.md":     background,
		"heldout.patch":  []byte(patch + "\n"),
		"reference.stat": []byte(stat + "\n"),
	})
}

// planAt returns planRepo's commit (any ref) to change's tasks.md with the latest author date no
// later than ref's in repo. Author dates, because a rebase re-stamps every plan commit's committer
// date after the code it planned.
func planAt(ctx context.Context, repo, ref, planRepo, change string) (string, error) {
	out, err := gitOut(ctx, repo, "log", "-1", "--format=%aI", ref)
	if err != nil {
		return "", err
	}
	when, err := time.Parse(time.RFC3339, out)
	if err != nil {
		return "", err
	}
	log, err := gitOut(ctx, planRepo, "log", "--all", "--format=%H %aI", "--", "spectre/changes/"+change+"/tasks.md")
	if err != nil {
		return "", err
	}
	var at string
	var latest time.Time
	for line := range strings.Lines(log) {
		sha, date, _ := strings.Cut(strings.TrimSpace(line), " ")
		t, err := time.Parse(time.RFC3339, date)
		if err != nil {
			return "", err
		}
		if !t.After(when) && t.After(latest) {
			at, latest = sha, t
		}
	}
	if at == "" {
		return "", fmt.Errorf("%s: %s has no commit of spectre/changes/%s/tasks.md authored by %s", change, planRepo, change, out)
	}
	return at, nil
}

// Review drafts root/review/<change>/ from change's round-0 review range in the checkout repo, its
// archive in planRepo (repo itself for a single-repository change): base is the parent of the
// oldest commit carrying a `Task-Id:` trailer reachable from head and from no branch when the change has a
// pre-reshape ref (see prereshape), else of the ledger's first implementer commit; head is the
// ledger's last implementer commit before the first panel-fix.
// diff.patch leaves out the change's plan files (decision review-cases-diff-only). Labels come
// from panel.md (see labelsFromPanel). It returns the draft directory, and refuses to overwrite an
// existing one.
func Review(ctx context.Context, root, repo, planRepo, change string) (string, error) {
	art := filepath.Join(planRepo, _archive, change)
	pin, fetchErr := prereshape(ctx, repo, change)
	ledger, err := readLedger(ctx, art, repo)
	if err != nil {
		return "", err
	}
	var first, head string
	for _, d := range ledger {
		if d.role == "panel-fix" {
			break
		}
		if d.role == "implementer" && d.commit != "" {
			first = cmp.Or(first, d.commit)
			head = d.commit
		}
	}
	if head == "" {
		return "", fmt.Errorf("%s: ledger.md has no implementer commit before the first panel-fix", change)
	}
	head, err = gitOut(ctx, repo, "rev-parse", "--verify", head+"^{commit}")
	if err != nil {
		return "", withFetch(err, fetchErr)
	}
	if pin != "" {
		found, err := taskCommits(ctx, repo, head, "")
		if err != nil {
			return "", err
		}
		if len(found) > 0 {
			first = found[0]
		}
	}
	base, err := gitOut(ctx, repo, "rev-parse", "--verify", first+"^")
	if err != nil {
		return "", err
	}
	noPlan := []string{"--", ".", ":(exclude)spectre/changes", ":(exclude)openspec/changes"}
	diff, err := gitOut(ctx, repo, append([]string{"diff", base, head}, noPlan...)...)
	if err != nil {
		return "", err
	}
	panel, err := os.ReadFile(filepath.Join(art, "panel.md"))
	if err != nil {
		return "", err
	}
	changed, err := gitOut(ctx, repo, append([]string{"diff", "--name-only", base, head}, noPlan...)...)
	if err != nil {
		return "", err
	}
	labels, err := marshal(labelsFromPanel(string(panel), filepath.Base(repo), strings.Split(changed, "\n")))
	if err != nil {
		return "", err
	}
	c := cases.Case{
		ID: change, Role: "review", Source: cases.Source{Repo: filepath.Base(repo), Change: change},
		Snapshot: head, Notes: fmt.Sprintf("round-0 range %s..%s", base, head),
	}
	return write(root, c, map[string][]byte{"diff.patch": []byte(diff + "\n"), "labels.json": labels})
}

// Visual drafts root/visual/<d.ID>/ from a mockup and capture pair, and labels.json holding the
// verdict and defects. The capture is copied as it is. Where the checkout holding the mockup
// declares a `mockup frame` geometry in .flow/project.md, the mockup is that board's frame, cut
// exactly as flow's visual verification cuts it (d.Compose), so it compares 1:1 with the capture at
// --scale 1 (decision visual-mockup-frame-crop); otherwise it is copied as it is. The source
// repository is the checkout holding the mockup. It returns the draft directory, and refuses to
// overwrite an existing one.
func Visual(ctx context.Context, root string, d VisualDraft) (string, error) {
	switch {
	case d.Verdict != "match" && d.Verdict != "mismatch":
		return "", fmt.Errorf("verdict %q is not match or mismatch", d.Verdict)
	case d.Verdict == "match" && len(d.Defects) > 0:
		return "", errors.New("a match verdict lists no defect")
	case d.Verdict == "mismatch" && len(d.Defects) == 0:
		return "", errors.New("a mismatch verdict needs at least one defect")
	case slices.ContainsFunc(d.Defects, func(df Defect) bool { return df.Summary == "" }):
		return "", errors.New("a defect needs a summary")
	}
	mockup, err := os.ReadFile(d.Mockup)
	if err != nil {
		return "", err
	}
	capture, err := os.ReadFile(d.Capture)
	if err != nil {
		return "", err
	}
	top, err := gitOut(ctx, filepath.Dir(d.Mockup), "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("mockup %s is not inside a source checkout: %w", d.Mockup, err)
	}
	if geometry, err := frameGeometry(top); err != nil {
		return "", err
	} else if geometry != "" {
		if mockup, err = cropFrame(ctx, d.Compose, geometry, d.Mockup, d.Capture); err != nil {
			return "", fmt.Errorf("crop mockup %s to its frame: %w", d.Mockup, err)
		}
	}
	labels, err := marshal(visualLabels{Verdict: d.Verdict, Defects: d.Defects})
	if err != nil {
		return "", err
	}
	prefix, err := gitOut(ctx, filepath.Dir(d.Mockup), "rev-parse", "--show-prefix")
	if err != nil {
		return "", err
	}
	rel := path.Join(prefix, filepath.Base(d.Mockup))
	c := cases.Case{
		ID: d.ID, Role: "visual", Source: cases.Source{Repo: filepath.Base(top)},
		Notes: fmt.Sprintf("mockup %s, capture %s", rel, filepath.Base(d.Capture)),
	}
	return write(root, c, map[string][]byte{"mockup.png": mockup, "capture.png": capture, "labels.json": labels})
}

var _frameRow = regexp.MustCompile("(?m)^\\|\\s*`mockup frame`\\s*\\|\\s*`([^`]+)`\\s*\\|")

// frameGeometry returns the `mockup frame` row of top's .flow/project.md — `scale=… status=…
// border=…` — or "" when the file or the row is absent.
func frameGeometry(top string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(top, ".flow", "project.md"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if m := _frameRow.FindSubmatch(raw); m != nil {
		return string(m[1]), nil
	}
	return "", nil
}

// cropFrame runs compose on the mockup board and the capture under geometry and returns the
// cropped frame it writes. compose refuses (exit 1) a capture whose size differs from the frame.
func cropFrame(ctx context.Context, compose, geometry, mockup, capture string) ([]byte, error) {
	if compose == "" {
		return nil, errors.New("the mockup's checkout declares a frame geometry but no compose-mockup-frames.sh was given")
	}
	tmp, err := os.MkdirTemp("", "flow-eval-crop-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	// compose pairs a map line's `<name>.png` with the stdin capture of that basename.
	raw, err := os.ReadFile(capture)
	if err != nil {
		return nil, err
	}
	shot := filepath.Join(tmp, "capture.png")
	frame := strings.TrimSuffix(filepath.Base(mockup), ".png")
	if err := os.WriteFile(shot, raw, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(tmp, "map"), []byte("capture.png "+frame+"\n"), 0o644); err != nil {
		return nil, err
	}
	out := filepath.Join(tmp, "out")
	cmd := exec.CommandContext(ctx, compose, filepath.Join(tmp, "map"), filepath.Dir(mockup), out, geometry)
	cmd.Stdin = strings.NewReader(shot + "\n")
	if msg, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", filepath.Base(compose), err, strings.TrimSpace(string(msg)))
	}
	return os.ReadFile(filepath.Join(out, "capture.frame.png"))
}

// write creates root/<role>/<id>/ — refusing one that exists or an id that is not one path
// element — and writes case.json and the assets into it.
func write(root string, c cases.Case, assets map[string][]byte) (string, error) {
	if c.ID == "" || c.ID != filepath.Base(c.ID) || strings.HasPrefix(c.ID, ".") {
		return "", fmt.Errorf("case id %q is not a single directory name", c.ID)
	}
	raw, err := marshal(c)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, c.Role, c.ID)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", fmt.Errorf("draft %s: %w", dir, err)
	}
	assets["case.json"] = raw
	for name, data := range assets {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// marshal renders v as indented JSON with a trailing newline.
func marshal(v any) ([]byte, error) {
	raw, err := json.Marshal(v, jsontext.WithIndent("  "))
	return append(raw, '\n'), err
}

// dispatch is one ledger.md row's fields that mining reads; commit is empty for "no commit".
type dispatch struct{ task, role, key, commit string }

// readLedger parses the dispatches of art/ledger.md in ledger order. A cross-repository commit map,
// `<worktree>=<sha>[,<worktree>=<sha>…]`, keeps the sha that is a commit in repo; a map with none
// there reads as no commit — that dispatch committed to another repository only.
func readLedger(ctx context.Context, art, repo string) ([]dispatch, error) {
	raw, err := os.ReadFile(filepath.Join(art, "ledger.md"))
	if err != nil {
		return nil, err
	}
	var out []dispatch
	for _, section := range strings.Split(string(raw), "\n## Dispatch ")[1:] {
		var d dispatch
		for line := range strings.Lines(section) {
			key, value, _ := strings.Cut(strings.TrimSpace(line), ": ")
			switch key {
			case "- Task":
				d.task = value
			case "- Role":
				d.role = value
			case "- Key":
				d.key = value
			case "- Commit":
				if value != "no commit" {
					d.commit = commitIn(ctx, repo, value)
				}
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// commitIn returns the sha of a ledger Commit value that names a commit in repo: the value itself
// when it is no map, else the first pair's sha that resolves there, or "" when none does.
func commitIn(ctx context.Context, repo, value string) string {
	if !strings.Contains(value, "=") {
		return value
	}
	for pair := range strings.SplitSeq(value, ",") {
		_, sha, _ := strings.Cut(strings.TrimSpace(pair), "=")
		if _, err := gitOut(ctx, repo, "rev-parse", "--verify", "--quiet", sha+"^{commit}"); err == nil {
			return sha
		}
	}
	return ""
}

var (
	_taskHead = regexp.MustCompile(`(?m)^- \[[ x]\] (\d+)\. `)
	_backtick = regexp.MustCompile("`([^`]+)`")
)

// taskSection returns task n of a tasks.md — from its checkbox line to the next task's — and the
// backticked paths of its **Files:** field.
func taskSection(tasks string, n int) (section string, files []string, err error) {
	heads := _taskHead.FindAllStringSubmatchIndex(tasks, -1)
	for i, h := range heads {
		if tasks[h[2]:h[3]] != strconv.Itoa(n) {
			continue
		}
		end := len(tasks)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		section = strings.TrimRight(tasks[h[0]:end], "\n") + "\n"
		for line := range strings.Lines(section) {
			if field, ok := strings.CutPrefix(line, "**Files:**"); ok {
				for _, m := range _backtick.FindAllStringSubmatch(field, -1) {
					files = append(files, m[1])
				}
				break
			}
		}
		if len(files) == 0 {
			return "", nil, fmt.Errorf("task %d in tasks.md has no **Files:** paths", n)
		}
		return section, files, nil
	}
	return "", nil, fmt.Errorf("tasks.md has no task %d", n)
}

// isTest reports whether a changed path is a test file the impl grader holds out.
func isTest(p string) bool {
	q := "/" + p
	return strings.Contains(q, "/src/test/") || strings.Contains(q, "/commonTest/") ||
		strings.Contains(q, "/desktopTest/") || strings.HasSuffix(p, "Test.kt") || strings.HasSuffix(p, "_test.go")
}

// verifyCommands derives an impl case's verify list from the checkout at base: the ## lint
// commands of .flow/project.md, then one --tests run per module for the held-out Kotlin and Java
// classes (grade.TestRuns), then each such module's test task. needsDocker is set when a module's
// build file names testcontainers. A held-out file outside src/<set>/ gets no selector; the
// operator adds one.
func verifyCommands(ctx context.Context, repo, base string, heldout []string) (verify []string, needsDocker bool, err error) {
	// A checkout declaring no ## lint — gymie-frontend, whose changes are planned in gymie — gets
	// none; the curator adds its lint commands by hand (docs/cases.md).
	if project, err := gitOut(ctx, repo, "show", base+":.flow/project.md"); err == nil {
		verify = lintCommands(project)
	}
	runs := grade.TestRuns(heldout)
	for _, r := range runs {
		verify = append(verify, r.Command())
	}
	for _, r := range runs {
		verify = append(verify, "./gradlew "+r.Task)
		for _, build := range []string{"build.gradle.kts", "build.gradle"} {
			body, err := gitOut(ctx, repo, "show", base+":"+path.Join(r.Dir, build))
			if err == nil && strings.Contains(strings.ToLower(body), "testcontainers") {
				needsDocker = true
			}
		}
	}
	return verify, needsDocker, nil
}

// lintCommands returns the commands of project.md's ## lint code block, trailing comments cut.
func lintCommands(project string) []string {
	_, section, ok := strings.Cut(project, "\n## lint\n")
	if !ok {
		return nil
	}
	section, _, _ = strings.Cut(section, "\n## ")
	_, block, ok := strings.Cut(section, "```bash\n")
	if !ok {
		return nil
	}
	block, _, _ = strings.Cut(block, "```")
	var out []string
	for line := range strings.Lines(block) {
		cmd, _, _ := strings.Cut(line, " #") // ponytail: a " #" inside a quoted argument is cut too
		if cmd = strings.TrimSpace(cmd); cmd != "" && !strings.HasPrefix(cmd, "#") {
			out = append(out, cmd)
		}
	}
	return out
}

// labelsFromPanel turns panel.md's findings table and finding-status lines into review labels for
// a case of repo whose diff.patch changes the files in diff. Every label carries the slot that
// raised it, from the table's Slot column. A fixed Critical or Important finding is mustFind,
// carrying its severity, even where it duplicates another slot's finding or cites a line outside
// the diff's hunks; a withdrawal whose reason says the premise was false is knownFalse. A finding
// the diff cannot show — a plan-alignment finding (decision review-cases-diff-only), one in another
// repository, or one at a file diff.patch does not change, as a later round's finding on fix code
// is — goes to ignored with that reason, as does every other withdrawal. Everything else is left out.
func labelsFromPanel(panel, repo string, diff []string) reviewLabels {
	type status struct{ state, reason string }
	statuses := map[string]status{}
	for line := range strings.Lines(panel) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "finding-status: ")
		if !ok {
			continue
		}
		id, rest, _ := strings.Cut(rest, " ")
		state, reason, _ := strings.Cut(rest, " ")
		statuses[id] = status{state: state, reason: strings.TrimSpace(reason)}
	}
	l := reviewLabels{MustFind: []grade.Label{}, KnownFalse: []grade.Label{}, Ignored: []IgnoredLabel{}}
	for line := range strings.Lines(panel) {
		cells := strings.Split(strings.TrimSpace(line), "|")
		if len(cells) < 7 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "F") {
			continue
		}
		id, severity := strings.TrimSpace(cells[1]), strings.TrimSpace(cells[3])
		in, file, lineNo := location(strings.TrimSpace(cells[4]))
		label := grade.Label{
			ID: id, Slot: strings.TrimSpace(cells[2]), File: file, Line: lineNo, Summary: strings.TrimSpace(cells[5]),
		}
		st := statuses[id]
		mustFind := st.state == "fixed" && (strings.EqualFold(severity, "Critical") || strings.EqualFold(severity, "Important"))
		if !mustFind && st.state != "withdrawn" {
			continue
		}
		switch {
		case isPlanArtifact(file):
			if st.state == "withdrawn" {
				l.Ignored = append(l.Ignored, IgnoredLabel{Label: label, Reason: "plan artifact: " + st.reason})
			}
		case in != "" && in != repo:
			l.Ignored = append(l.Ignored, IgnoredLabel{Label: label, Reason: "in repository " + in + ", not " + repo})
		case !slices.Contains(diff, file):
			l.Ignored = append(l.Ignored, IgnoredLabel{Label: label, Reason: file + " is not in diff.patch"})
		case mustFind:
			label.Severity = strings.ToUpper(severity[:1]) + strings.ToLower(severity[1:])
			l.MustFind = append(l.MustFind, label)
		case _falsePremise.MatchString(st.reason):
			l.KnownFalse = append(l.KnownFalse, label)
		default:
			l.Ignored = append(l.Ignored, IgnoredLabel{Label: label, Reason: st.reason})
		}
	}
	return l
}

// _falsePremise matches a withdrawal reason saying the finding was never a defect.
var _falsePremise = regexp.MustCompile(`(?i)premise (?:was |is )?false|false premise|not a defect|false positive`)

var _locLine = regexp.MustCompile(`^:(\d+)`)

// location splits a panel Location — `[repo:]path[:line[,…|-…]][ (note)]` — into its repository
// prefix ("" when none), path and first line (0 when it names none).
func location(loc string) (repo, file string, line int) {
	// A repository prefix has no / or . and is followed by a path, never by a line number.
	if r, rest, ok := strings.Cut(loc, ":"); ok && !strings.ContainsAny(r, "/.") && !_locLine.MatchString(":"+rest) {
		repo, loc = r, rest
	}
	loc, _, _ = strings.Cut(loc, " ")
	file, rest, ok := strings.Cut(loc, ":")
	if !ok {
		return repo, file, 0
	}
	if m := _locLine.FindStringSubmatch(":" + rest); m != nil {
		line, _ = strconv.Atoi(m[1])
	}
	return repo, file, line
}

// isPlanArtifact reports whether a finding's file is one of a change's plan artifacts.
func isPlanArtifact(file string) bool {
	return strings.HasPrefix(file, "spectre/changes/") || strings.HasPrefix(file, "openspec/changes/")
}

// _fixSuffix is a sub-change's `-fix-<N>` suffix: a fix run carries no pin of its own.
var _fixSuffix = regexp.MustCompile(`-fix-\d+$`)

// prereshape returns refs/flow/pre-reshape/<change>, the change branch as integrate pinned it before
// squashing its task commits, once repo has it — fetched from origin when repo lacks it — or ""
// when origin has none either, with the fetch's error. A `<name>-fix-<N>` change with no pin of its
// own falls back to <name>'s.
func prereshape(ctx context.Context, repo, change string) (string, error) {
	ref := "refs/flow/pre-reshape/" + change
	if _, err := gitOut(ctx, repo, "rev-parse", "--verify", "--quiet", ref); err == nil {
		return ref, nil
	}
	_, err := gitOut(ctx, repo, "fetch", "--quiet", "origin", ref+":"+ref)
	if err == nil {
		return ref, nil
	}
	if parent := _fixSuffix.ReplaceAllString(change, ""); parent != change {
		if pin, perr := prereshape(ctx, repo, parent); pin != "" {
			return pin, nil
		} else if perr != nil {
			err = perr
		}
	}
	return "", err
}

// taskCommits returns the commits reachable from until and from no branch or remote-tracking ref
// carrying a `Task-Id: <n>` trailer — any task's when n is "" — oldest first. The bound is the
// same whatever the checkout's HEAD; pin, a refs/flow/ ref, is never in the excluded set.
func taskCommits(ctx context.Context, repo, until, n string) ([]string, error) {
	id := cmp.Or(n, "[0-9][0-9]*")
	out, err := gitOut(ctx, repo, "log", "--reverse", "--format=%H", "--grep=^Task-Id: "+id+"$", until, "--not", "--branches", "--remotes")
	return strings.Fields(out), err
}

// withFetch adds a failed pin fetch to err, the ledger sha's resolution failure it may explain.
func withFetch(err, fetch error) error {
	if fetch == nil {
		return err
	}
	return fmt.Errorf("%w (fetching the pre-reshape ref: %v)", err, fetch)
}

// gitOut runs a git command — read-only, save prereshape's fetch — in dir and returns its stdout without the trailing newline.
func gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "--no-pager"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(string(out), "\n"), nil
}
