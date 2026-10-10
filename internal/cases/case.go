// Package cases loads and validates eval cases: cases/<role>/<id>/case.json plus the role's assets
// (design.md §3).
package cases

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Source names where a case was mined from.
type Source struct {
	Repo   string `json:"repo"`
	Change string `json:"change"`
	URL    string `json:"url,omitempty"`
}

// Case is one case.json. Snapshot applies to impl and review cases: the source repository commit a
// trial's /work is a snapshot of — an impl case's base, a review case's reviewed head. Verify and
// Files apply to impl cases only: Verify is the grader's command list, Files the task's declared
// scope.
type Case struct {
	ID          string   `json:"id"`
	Role        string   `json:"role"`
	Source      Source   `json:"source"`
	Snapshot    string   `json:"snapshot,omitempty"`
	Notes       string   `json:"notes"`
	NeedsDocker bool     `json:"needsDocker"`
	Verify      []string `json:"verify,omitempty"`
	Files       []string `json:"files,omitempty"`

	// Dir is the case directory the assets live in; set by the loader, never read from JSON.
	Dir string `json:"-"`
}

// requiredAssets lists, per role, the files a case directory must hold besides case.json. Its keys
// are the known roles.
var requiredAssets = map[string][]string{
	"impl":   {"task.md", "context.md", "heldout.patch", "reference.stat"},
	"review": {"diff.patch", "labels.json"},
	"visual": {"mockup.png", "capture.png", "labels.json"},
}

func roleNames() string {
	return strings.Join(slices.Sorted(maps.Keys(requiredAssets)), ", ")
}

func byRoleThenID(a, b Case) int {
	return cmp.Or(cmp.Compare(a.Role, b.Role), cmp.Compare(a.ID, b.ID))
}

// LoadAll loads every case under root/<role>/<id>/ — under every role directory when role is "" —
// sorted by role, then id. A case that fails validation is returned in invalid, one error per case
// naming its directory and every reason. err is reserved for a role or root that cannot be used at
// all.
func LoadAll(root, role string) (cases []Case, invalid []error, err error) {
	roleDirs := []string{role}
	if role == "" {
		if roleDirs, err = subdirs(root); err != nil {
			return nil, nil, err
		}
	} else if _, ok := requiredAssets[role]; !ok {
		return nil, nil, fmt.Errorf("unknown role %q: want one of %s", role, roleNames())
	}
	for _, rd := range roleDirs {
		ids, err := subdirs(filepath.Join(root, rd))
		if errors.Is(err, fs.ErrNotExist) && role != "" {
			if _, rootErr := os.Stat(root); rootErr != nil {
				return nil, nil, rootErr
			}
			continue // the root exists but holds no case of this role yet
		}
		if err != nil {
			return nil, nil, err
		}
		for _, id := range ids {
			dir := filepath.Join(root, rd, id)
			c, problems := load(dir, rd, id)
			if len(problems) > 0 {
				invalid = append(invalid, fmt.Errorf("%s: %s", dir, strings.Join(problems, "; ")))
				continue
			}
			cases = append(cases, c)
		}
	}
	slices.SortFunc(cases, byRoleThenID)
	return cases, invalid, nil
}

// subdirs lists the non-hidden directories, and symlinks (which load refuses), directly under dir,
// sorted.
func subdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if (e.IsDir() || e.Type()&fs.ModeSymlink != 0) && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// load reads and validates the case in dir, found under role directory roleDir as id. It returns
// every problem found, not only the first.
func load(dir, roleDir, id string) (Case, []string) {
	c := Case{Dir: dir}
	if fi, err := os.Lstat(dir); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		return c, []string{"a case directory must not be a symlink"}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "case.json"))
	if err != nil {
		return c, []string{err.Error()}
	}
	if err := json.Unmarshal(raw, &c, json.RejectUnknownMembers(true)); err != nil {
		return c, []string{"case.json: " + err.Error()}
	}
	var problems []string
	if c.ID != id {
		problems = append(problems, fmt.Sprintf("id %q does not match its directory %q", c.ID, id))
	}
	assets, known := requiredAssets[c.Role]
	switch {
	case !known:
		problems = append(problems, fmt.Sprintf("role %q is not one of %s", c.Role, roleNames()))
	case c.Role != roleDir:
		problems = append(problems, fmt.Sprintf("role %q does not match its role directory %q", c.Role, roleDir))
	}
	if c.Source.Repo == "" {
		problems = append(problems, "source.repo is empty")
	}
	if c.Role == "impl" || c.Role == "review" {
		if c.Source.Change == "" {
			problems = append(problems, "source.change is empty")
		}
		if c.Snapshot == "" {
			problems = append(problems, "snapshot is empty: the trial's repository commit is required")
		}
	}
	if c.Role == "impl" {
		if len(c.Verify) == 0 {
			problems = append(problems, "verify is empty: the grader would pass a trial without running anything")
		}
		if len(c.Files) == 0 {
			problems = append(problems, "files is empty: the task's declared scope is required")
		}
	}
	for _, a := range assets {
		if fi, err := os.Stat(filepath.Join(dir, a)); err != nil || !fi.Mode().IsRegular() {
			problems = append(problems, "missing asset "+a)
		}
	}
	if slices.Contains(assets, "labels.json") {
		problems = append(problems, checkLabels(c.Role, filepath.Join(dir, "labels.json"))...)
	}
	return c, problems
}

// checkLabels checks that labels.json parses, for review that every label carries the slot that
// raised it, and for visual that the verdict and defects agree. A missing file is already reported
// as a missing asset.
func checkLabels(role, path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var l struct {
		Verdict string `json:"verdict"`
		Defects []struct {
			Summary string `json:"summary"`
		} `json:"defects"`
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return []string{"labels.json: " + err.Error()}
	}
	if role == "review" {
		return checkSlots(raw)
	}
	if role != "visual" {
		return nil
	}
	switch l.Verdict {
	case "match":
		if len(l.Defects) > 0 {
			return []string{"labels.json: a match verdict lists defects"}
		}
	case "mismatch":
		if len(l.Defects) == 0 {
			return []string{"labels.json: a mismatch verdict lists no defect"}
		}
		for i, d := range l.Defects {
			if d.Summary == "" {
				return []string{fmt.Sprintf("labels.json: defect %d has no summary", i)}
			}
		}
	default:
		return []string{fmt.Sprintf("labels.json: verdict %q is not match or mismatch", l.Verdict)}
	}
	return nil
}

// checkSlots returns a problem naming every review label of raw that carries no slot: a case mined
// before labels carried one, which the grader cannot grade per slot.
func checkSlots(raw []byte) []string {
	type label struct {
		ID   string `json:"id"`
		Slot string `json:"slot"`
	}
	var l struct {
		MustFind   []label `json:"mustFind"`
		KnownFalse []label `json:"knownFalse"`
		Ignored    []label `json:"ignored"`
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return []string{"labels.json: " + err.Error()}
	}
	var bare []string
	for _, lb := range slices.Concat(l.MustFind, l.KnownFalse, l.Ignored) {
		if lb.Slot == "" {
			bare = append(bare, lb.ID)
		}
	}
	if len(bare) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("labels.json: labels %s carry no slot; re-mine the case with flow-eval case add review",
		strings.Join(bare, ", "))}
}

// SetHash identifies a case set: sha256 over each case's role and id and every non-hidden file in
// its directory (slash-separated relative path, length and contents), cases sorted by role then id and
// files in lexical order, so neither input order nor directory read order changes it.
func SetHash(cases []Case) (string, error) {
	sorted := slices.Clone(cases)
	slices.SortFunc(sorted, byRoleThenID)
	h := sha256.New()
	for _, c := range sorted {
		fmt.Fprintf(h, "case %s/%s\x00", c.Role, c.ID)
		err := filepath.WalkDir(c.Dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path != c.Dir && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil // .DS_Store and the like are not case content
			}
			if d.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(c.Dir, path)
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "file %s %d\x00", filepath.ToSlash(rel), len(data))
			h.Write(data)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("hash case %s/%s: %w", c.Role, c.ID, err)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
