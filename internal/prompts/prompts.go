// Package prompts vendors flow's role prompts from a pinned agents sha and renders them for a trial
// (design.md §4). prompts/strip.txt lists, per target, the sections removed from the upstream text;
// prompts/PIN records the sha the vendored files came from.
package prompts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

// target is one vendored prompt: prompts/<name>.md, cut from src in the agents repository.
type target struct {
	name    string
	src     string
	extract func(src string) (string, error)
	// appendix is a further agents file vendored whole after the stripped body, under its own
	// heading; empty for none.
	appendix string
	// fill maps each upstream placeholder the stripped body keeps to the template field a trial fills
	// it with; sync fails when one is absent, so upstream drift is seen.
	fill map[string]string
	tail string // the role's output contract and input slots, appended last
}

// _reviewTail is the output slot every review target appends: the diff under review, inline.
const _reviewTail = "## The diff under review\n\n{{.Diff}}\n"

var _targets = []target{
	{
		name:    "impl",
		src:     "skills/flow/implement.md",
		extract: dispatchBlocks,
		tail: "## Your task\n\n{{.Task}}\n\n## Context\n\n{{.Context}}\n\n## When you are done\n\n" +
			"Implement the task in the current directory, which is the repository. Files the rules above " +
			"cite by path (under `~/.claude/` or `skills/`) are not available here; apply each rule as " +
			"stated. Reply with a short report: what you changed, the tests you added, and each command " +
			"you ran with its result.\n",
	},
	{
		name:     "review-primary",
		src:      "skills/flow/primary-reviewer-prompt.md",
		extract:  fencedPrompt,
		appendix: "skills/flow/reviewer-calibration.md",
		tail:     _reviewTail,
	},
	{
		name:     "review-principles",
		src:      "skills/flow/principles-reviewer-prompt.md",
		extract:  fencedPrompt,
		appendix: "skills/flow/engineering-principles.md",
		fill:     map[string]string{"[STANDARDS_PATHS]": "{{.Standards}}"},
		tail:     _reviewTail,
	},
	{
		name:    "review-failure-modes",
		src:     "skills/flow/failure-modes-reviewer-prompt.md",
		extract: fencedPrompt,
		tail:    _reviewTail,
	},
	{
		name:    "visual",
		src:     "skills/flow/visual-verify-verifier.md",
		extract: func(src string) (string, error) { return src, nil },
		tail: "## The pair to judge\n\n- Mockup: {{.Mockup}}\n- Capture: {{.Capture}}\n\n" +
			"Read both images and judge whether the capture matches the mockup. " +
			"`measure-visual-properties.sh` is on PATH. The mockup is already cut to its frame, the same " +
			"size as the capture, so measure the two with `--scale 1`; nothing is launched, captured, " +
			"composed or written to a report file.\n\n## Verdict\n\n" +
			"List each departure on its own line as `- defect: <what differs, and where>`, then end " +
			"your reply with exactly one line: `verdict: match` or `verdict: mismatch`.\n",
	},
}

// The implementer dispatch blocks are the blockquotes of implement.md between these two lines.
const (
	_implFrom = "Every implementer dispatch **must** carry:"
	_implTo   = "**The parent runs the full suite — never an implementer.**"
)

var (
	_heading  = regexp.MustCompile(`^#{1,6} `)
	_numbered = regexp.MustCompile(`^\d+\. \*\*`)
)

// Strip removes, in order, each section whose opening line starts with one of headings. A section
// opens at a Markdown heading, a numbered bold item (`4. **…`), or a bold-led line that follows a
// blank line or is dedented from the line before; it runs to the next opening line of any kind, so
// a heading's section ends at its first bold-led paragraph and strip.txt lists each part to drop.
// Nothing inside a code fence opens a section. An entry led by `~` instead cuts that literal
// clause, matched across line wraps, from inside a section the procedure keeps, with the whitespace
// before it — or, when the clause opens a line, with whichever of the whitespace before and after it
// holds fewer line breaks (the after, on a tie), so the line break before it, a blank line after it,
// and the text after it stay. A heading that opens no section, or a clause found nowhere,
// is an error, so upstream drift is seen rather than silently ignored.
func Strip(doc string, headings []string) (string, error) {
	for _, h := range headings {
		if clause, ok := strings.CutPrefix(h, "~"); ok {
			words := strings.Fields(clause)
			for i, w := range words {
				words[i] = regexp.QuoteMeta(w)
			}
			re := regexp.MustCompile(`(\s*)` + strings.Join(words, `\s+`) + `(\s*)`)
			if !re.MatchString(doc) {
				return "", fmt.Errorf("strip: no clause %q", clause)
			}
			doc = re.ReplaceAllStringFunc(doc, func(m string) string {
				ws := re.FindStringSubmatch(m)
				if before := strings.Count(ws[1], "\n"); before > 0 && strings.Count(ws[2], "\n") <= before {
					return ws[1]
				}
				return ws[2]
			})
			continue
		}
		lines := strings.Split(doc, "\n")
		starts := sectionStarts(lines)
		var kept []string
		found, dropping := false, false
		for i, l := range lines {
			if starts[i] {
				dropping = strings.HasPrefix(strings.TrimLeft(l, " "), h)
				found = found || dropping
			}
			if !dropping {
				kept = append(kept, l)
			}
		}
		if !found {
			return "", fmt.Errorf("strip: no section opens with %q", h)
		}
		doc = strings.Join(kept, "\n")
	}
	return doc, nil
}

// sectionStarts reports, per line, whether the line opens a section (see Strip).
func sectionStarts(lines []string) []bool {
	starts := make([]bool, len(lines))
	inFence, prevBlank, prevIndent := false, true, 0
	for i, l := range lines {
		text := strings.TrimLeft(l, " ")
		indent := len(l) - len(text)
		switch {
		case strings.HasPrefix(text, "```"):
			inFence = !inFence
		case inFence:
		case text == "":
			prevBlank = true
			continue
		case _heading.MatchString(text), _numbered.MatchString(text):
			starts[i] = true
		case strings.HasPrefix(text, "**"):
			starts[i] = prevBlank || indent < prevIndent
		}
		prevBlank, prevIndent = false, indent
	}
	return starts
}

// dispatchBlocks returns implement.md's implementer dispatch blocks, unquoted, one blank line
// between blocks.
func dispatchBlocks(src string) (string, error) {
	_, rest, ok := strings.Cut(src, "\n"+_implFrom+"\n")
	if !ok {
		return "", fmt.Errorf("no line %q", _implFrom)
	}
	rest, _, ok = strings.Cut(rest, "\n"+_implTo)
	if !ok {
		return "", fmt.Errorf("no line %q", _implTo)
	}
	var out []string
	for l := range strings.SplitSeq(rest, "\n") {
		quoted, ok := strings.CutPrefix(l, ">")
		if !ok {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		out = append(out, strings.TrimPrefix(quoted, " "))
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n", nil
}

// fencedPrompt returns the body of src's first code fence with its four-space indent removed: each
// reviewer prompt is written as one indented fenced block.
func fencedPrompt(src string) (string, error) {
	_, rest, ok := strings.Cut(src, "```\n")
	if !ok {
		return "", errors.New("no code fence")
	}
	body, _, ok := strings.Cut(rest, "\n```")
	if !ok {
		return "", errors.New("unclosed code fence")
	}
	var out []string
	for l := range strings.SplitSeq(body, "\n") {
		out = append(out, strings.TrimPrefix(l, "    "))
	}
	return strings.Join(out, "\n") + "\n", nil
}

// Sync vendors every target from agentsRepo at sha into dir: it reads each source with git show,
// removes the sections dir/strip.txt lists for it, fills its kept placeholders, appends its appendix
// file and the role's output contract and input slots, and writes dir/<target>.md. It then replaces
// dir/rules with every rules/*.mdc at sha — the shared rule library a `## standards` entry may name
// (Standards) — and writes dir/PIN. Nothing is written unless every target succeeds, and PIN is
// written last, so a failed write never pins stale prompts.
func Sync(ctx context.Context, dir, agentsRepo, sha string) error {
	full, err := git(ctx, agentsRepo, "rev-parse", "--verify", "--end-of-options", sha+"^{commit}")
	if err != nil {
		return fmt.Errorf("prompts sync: %w", err)
	}
	full = strings.TrimSpace(full)
	strip, err := loadStrip(filepath.Join(dir, "strip.txt"))
	if err != nil {
		return fmt.Errorf("prompts sync: %w", err)
	}
	type file struct{ name, body string }
	var files []file
	for _, t := range _targets {
		src, err := git(ctx, agentsRepo, "show", full+":"+t.src)
		if err != nil {
			return fmt.Errorf("prompts sync %s: %w", t.name, err)
		}
		body, err := t.extract(src)
		if err != nil {
			return fmt.Errorf("prompts sync %s: %s: %w", t.name, t.src, err)
		}
		if body, err = Strip(body, strip[t.name]); err != nil {
			return fmt.Errorf("prompts sync %s: %s: %w", t.name, t.src, err)
		}
		for from, to := range t.fill {
			if !strings.Contains(body, from) {
				return fmt.Errorf("prompts sync %s: %s: no placeholder %s", t.name, t.src, from)
			}
			body = strings.ReplaceAll(body, from, to)
		}
		out := strings.TrimRight(body, "\n") + "\n\n"
		if t.appendix != "" {
			extra, err := git(ctx, agentsRepo, "show", full+":"+t.appendix)
			if err != nil {
				return fmt.Errorf("prompts sync %s: %w", t.name, err)
			}
			out += "## Appendix: " + t.appendix + "\n\n" + strings.TrimRight(extra, "\n") + "\n\n"
		}
		out += t.tail
		if _, err := template.New(t.name).Parse(out); err != nil {
			return fmt.Errorf("prompts sync %s: vendored text is not a valid template: %w", t.name, err)
		}
		files = append(files, file{t.name + ".md", out})
	}
	tree, err := git(ctx, agentsRepo, "ls-tree", full, "rules/")
	if err != nil {
		return fmt.Errorf("prompts sync: %w", err)
	}
	var rules []file
	for l := range strings.Lines(tree) {
		meta, path, _ := strings.Cut(strings.TrimSuffix(l, "\n"), "\t")
		name := strings.TrimPrefix(path, "rules/")
		// A regular file only: a symlink's blob is its target's path, never a rule.
		mode, _, _ := strings.Cut(meta, " ")
		if !strings.HasSuffix(name, ".mdc") || (mode != "100644" && mode != "100755") {
			continue
		}
		body, err := git(ctx, agentsRepo, "show", full+":"+path)
		if err != nil {
			return fmt.Errorf("prompts sync: %w", err)
		}
		rules = append(rules, file{filepath.Join("rules", name), body})
	}
	if err := os.RemoveAll(filepath.Join(dir, "rules")); err != nil {
		return fmt.Errorf("prompts sync: %w", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "rules"), 0o755); err != nil {
		return fmt.Errorf("prompts sync: %w", err)
	}
	for _, f := range append(append(files, rules...), file{"PIN", full + "\n"}) {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.body), 0o644); err != nil {
			return fmt.Errorf("prompts sync: %w", err)
		}
	}
	return nil
}

// loadStrip reads strip.txt: one `<target>: <heading>` per line, in removal order; blank lines and
// lines starting with # are skipped.
func loadStrip(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	var names []string
	for _, t := range _targets {
		known[t.name] = true
		names = append(names, t.name)
	}
	strip := map[string][]string{}
	for n, l := range strings.Split(string(data), "\n") {
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		name, heading, ok := strings.Cut(l, ": ")
		if !ok || !known[name] || heading == "" {
			return nil, fmt.Errorf("%s:%d: want `<target>: <heading>` with a target of %s", path, n+1, strings.Join(names, ", "))
		}
		strip[name] = append(strip[name], heading)
	}
	return strip, nil
}

func git(ctx context.Context, repo string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// file is target's prompt file in dir.
func file(dir, target string) string { return filepath.Join(dir, target+".md") }

// Vendored returns an error unless dir holds target's prompt under exactly its name. It lists dir
// rather than stat the file, because a case-insensitive filesystem resolves a mis-cased name.
func Vendored(dir, target string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	name := filepath.Base(file(dir, target))
	for _, e := range entries {
		if e.Name() == name && !e.IsDir() {
			return nil
		}
	}
	return fmt.Errorf("no %s in %s", name, dir)
}

// Render fills dir/<target>.md's {{.Name}} slots from inputs. An input the prompt uses but inputs
// lacks is an error, never an empty slot.
func Render(dir, target string, inputs map[string]string) (string, error) {
	text, err := os.ReadFile(file(dir, target))
	if err != nil {
		return "", fmt.Errorf("render %s: %w", target, err)
	}
	tmpl, err := template.New(target).Option("missingkey=error").Parse(string(text))
	if err != nil {
		return "", fmt.Errorf("render %s: %w", target, err)
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, inputs); err != nil {
		return "", fmt.Errorf("render %s: %w", target, err)
	}
	return b.String(), nil
}

// Standard is one resolved `## standards` entry: the file on this machine, and the absolute path a
// trial container reads it at.
type Standard struct{ Host, Container string }

// _rulesInContainer is where a trial container holds the agents rules a project's standards name:
// outside /work, so the repository under review stays as the snapshot left it.
const _rulesInContainer = "/agents/rules/"

// Standards resolves the `## standards` entries of the project at root — a case snapshot's tree,
// which a trial sees at /work — as flow resolves them for its principles reviewer (agents
// skills/flow-contracts/project-configuration-standards.md). An entry is a top-level `- ` item of
// that section of root/.flow/project.md, its first backticked span or else its first word; the
// prose after it is the reader's. A bare `*.mdc` name resolves to dir/rules/<name>, the agents rule
// vendored at dir/PIN; any other bare name to root/<name>, a relative path against root, and an
// absolute path as it is. The entries are attacker-influenced, so each is resolved with its `..`
// and symlinks normalized and kept only when the result is a regular file inside its root — for a
// bare name, directly in it — named with the exact case it has on disk; every other entry is
// returned in dropped, by name. No .flow/project.md — a symlinked one included, since flow reads
// HEAD's blob of it and a symlink's blob is its target path — one outside root, or no `## standards`,
// resolves none; a section declared twice is an error, as flow refuses it.
//
// The section scan ports flow's project-get at the pin — stats/internal/guard/projectget.go and
// stats/internal/guard/projectsection.go in the agents repository, the Go twins of
// scripts/project-get.sh (which skills/flow/scripts/project-get.sh links to) and
// scripts/lib/project-section.sh — so a pin move re-checks them: a leading UTF-8 BOM dropped, the
// heading matched as the exact line `## standards`, the section ending at the next line opening `## `.
func Standards(dir, root string) (std []Standard, dropped []string, err error) {
	// Absolute, so filepath.Rel in within can relate an absolute entry to either root.
	if dir, err = filepath.Abs(dir); err != nil {
		return nil, nil, fmt.Errorf("standards: %w", err)
	}
	if root, err = filepath.Abs(root); err != nil {
		return nil, nil, fmt.Errorf("standards: %w", err)
	}
	rules := filepath.Join(dir, "rules")
	project := filepath.Join(".flow", "project.md")
	if fi, err := os.Lstat(filepath.Join(root, project)); err != nil || !fi.Mode().IsRegular() {
		return nil, nil, nil
	}
	if _, ok := within(root, filepath.Join(root, project), false); !ok {
		return nil, nil, nil
	}
	data, err := os.ReadFile(filepath.Join(root, project))
	if err != nil {
		return nil, nil, fmt.Errorf("standards: %w", err)
	}
	var entries []string
	in, seen := false, false
	for l := range strings.Lines(strings.TrimPrefix(string(data), "\ufeff")) {
		l = strings.TrimSuffix(l, "\n")
		if strings.HasPrefix(l, "## ") {
			in = l == "## standards"
			if in && seen {
				return nil, nil, fmt.Errorf("standards: %s declares ## standards more than once", project)
			}
			seen = seen || in
			continue
		}
		item, ok := strings.CutPrefix(l, "- ")
		if !in || !ok {
			continue
		}
		item = strings.TrimSpace(item)
		if code, ok := strings.CutPrefix(item, "`"); ok {
			item, _, _ = strings.Cut(code, "`")
		} else if f := strings.Fields(item); len(f) > 0 {
			item = f[0]
		}
		if item != "" {
			entries = append(entries, item)
		}
	}
	for _, e := range entries {
		if s, ok := resolve(e, root, rules); ok {
			std = append(std, s)
		} else {
			dropped = append(dropped, e)
		}
	}
	return std, dropped, nil
}

// resolve resolves one `## standards` entry per its form (see Standards).
func resolve(entry, root, rules string) (Standard, bool) {
	sep := string(filepath.Separator)
	inProject := func(cand string, direct bool) (Standard, bool) {
		rel, ok := within(root, cand, direct)
		return Standard{Host: filepath.Join(root, rel), Container: "/work/" + filepath.ToSlash(rel)}, ok
	}
	inRules := func(cand string, direct bool) (Standard, bool) {
		rel, ok := within(rules, cand, direct)
		return Standard{Host: filepath.Join(rules, rel), Container: _rulesInContainer + filepath.ToSlash(rel)}, ok
	}
	switch {
	case !strings.Contains(entry, "/") && strings.HasSuffix(entry, ".mdc"):
		return inRules(rules+sep+entry, true)
	case !strings.Contains(entry, "/"):
		return inProject(root+sep+entry, true)
	case filepath.IsAbs(entry):
		if s, ok := inProject(entry, false); ok {
			return s, true
		}
		return inRules(entry, false)
	default:
		return inProject(root+sep+entry, false)
	}
}

// within returns cand's path relative to base, both with their symlinks resolved, when cand is a
// regular file inside base — directly inside it when direct — and every component of that path
// names its directory entry in the exact case, so a case-insensitive host never resolves a path a
// Linux container lacks.
func within(base, cand string, direct bool) (string, bool) {
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", false
	}
	real, err := filepath.EvalSymlinks(cand)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(realBase, real)
	sep := string(filepath.Separator)
	escapes := rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+sep)
	if err != nil || escapes || (direct && strings.Contains(rel, sep)) {
		return "", false
	}
	if fi, err := os.Stat(real); err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	dir := realBase
	for _, name := range strings.Split(rel, sep) {
		entries, err := os.ReadDir(dir)
		if err != nil || !slices.ContainsFunc(entries, func(e os.DirEntry) bool { return e.Name() == name }) {
			return "", false
		}
		dir = filepath.Join(dir, name)
	}
	return rel, true
}
