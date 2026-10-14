# kan-978-tune-principles-prompt — design

## Context

- The case's principles must-find is F6 (`labels.json`): `SettingsState.kt:339`, `loadFailed` drifts from the facts it summarises. Every principles trial so far reported 0 recall on it.
- Primary's fix (agents `2e053aaa`) is the model: a numbered required step done before anything else, phrased generically, plus an output section that forces the reviewer to list its evidence (`path:line` + verdict). Measured across models in its commit message.
- Two repositories: agents holds the canonical prompt; flow-eval vendors it with `flow-eval prompts sync --agents <dir> --sha <sha>` and must rebuild `flow-eval-runner` afterwards (its build refuses another pin's archive).
- A review run calls the judge on the host and needs `CLAUDE_CODE_OAUTH_TOKEN` or `ANTHROPIC_API_KEY` in the environment.

## The prompt step

Variant 1 (the starting point; variants 2–3 revise its wording only if it scores 0/6 or another wording scores higher):

- **Stored state that summarises other state — a required step, done before anything else:**
  1. List every field, flag or variable the diff writes whose value could be computed from other state (a "something failed", "any unconfirmed", "is dirty", a count or total).
  2. For each, grep its file for every line that writes it, unchanged lines included, and name the facts it summarises.
  3. A writer that sets or clears it from only part of those facts — one field's path deciding for all of them — is a Single Source of Truth violation: the value should be derived, not stored. Report it as Important at least, at that writer.
- Output: `### Derived-state check` before `### Issues` — each state, its writers as `path:line`, the facts it summarises, and a verdict per writer.
- Never names anything from the case (`loadFailed`, settings, units).

## Measuring

- Candidate: `--harness claude-code --model claude-haiku-5-5 --effort low --role review --slot principles -k 6`.
- Before run: the current prompt (`2e053aaa`) at k=6, so before and after share k.
- Each variant: one agents commit, `prompts sync` into the flow-eval worktree, runner rebuilt, one k=6 run. At most three variants.
- Finds = trials whose judge matched F6 (recall 1).

## Decisions

### Where the prompt change lives

**ID:** agents-canonical-vendored
**Status:** active
**Chosen:** edit agents `skills/flow/principles-reviewer-prompt.md`, vendor into flow-eval with `prompts sync` — the path primary took; agents stays the single source of the prompt.
**Considered:** edit `prompts/review-principles.md` in flow-eval and upstream by hand — the vendored copy would diverge from its pin, and `prompts sync` would silently overwrite it.

### When tuning stops

**ID:** best-of-three-variants
**Status:** active
**Chosen:** at most three variants, one haiku-low k=6 run each; land the best that finds F6 at least once — chosen by the operator.
**Considered:** iterate until ≥3/6 finds — unbounded cost and a stronger pull to overfit one case; a single variant — no chance to correct a wording that misses.

### Measuring model and k

**ID:** haiku-low-k6
**Status:** active
**Chosen:** haiku-low at k=6, plus a same-k before run on the current prompt — the operator's "max k=6, try on haiku-low"; the before run makes 0/3 → x/6 a like-for-like comparison.
**Considered:** reuse the existing k=3 haiku run as the before figure — mixed k makes the counts incomparable; measure sonnet and opus too — not asked for.

### How variants are versioned

**ID:** variant-commits-on-a-pushed-branch
**Status:** active
**Chosen:** each variant is a commit on agents branch `kan-978-principles-variants` (pushed, so every run's `prompt pin` resolves); the winner is cherry-picked as the change's one agents commit, whose body lists every variant's sha and finds/6; flow-eval commits every variant's run files, each carrying its pin.
**Considered:** variants as successive commits on the change branch — the per-task commit guard expects one commit per task, and a losing variant would ship in history as if it were the prompt; variants as uncommitted edits — `prompts sync` needs a sha, so a run could not say what it measured.

### Baselines

**ID:** baselines-unchanged
**Status:** active
**Chosen:** leave `baselines/review-principles.json` (opus-low) as it is — a haiku run is a candidate against it, not its replacement.
**Considered:** set the winning haiku run as the baseline — replaces an opus reference with a cheaper model's, which nobody asked for.

### Where the guide lives

**ID:** guide-in-docs
**Status:** active
**Chosen:** `docs/prompt-tuning.md`, linked from README `## Commands` › `prompts sync` — beside `docs/cases.md`, the existing how-to; the operator asked for the repo, not memory.
**Considered:** a README section — the README is reference for commands, and the loop is a multi-step how-to like `docs/cases.md`; an agents skill — the loop is flow-eval's workflow and needs its commands.

### Where the eval credential comes from

**ID:** eval-credential-from-zshrc
**Status:** active
**Chosen:** every run command is wrapped in `zsh -ic '…'`, which loads `CLAUDE_CODE_OAUTH_TOKEN` from the operator's `~/.zshrc` — the operator's answer; verified SET there, and the value never enters a command line or record.
**Considered:** asking for a token file — unneeded once the operator named where it already lives.

### A label at more than one location

**ID:** label-also-lines
**Status:** active
**Chosen:** a review label carries an optional `also: [<line>, …]` — further lines of its `file` where the same defect sits; a finding near the label's `line` or any `also` line is put to the judge, and the judge is shown every line. F6 gets `also: [295]`. Refines kan-964's `review-match-location-plus-judge`: the window and the judge stay as they are.
**Considered:** moving F6's `line` to `:295` — moves the blind spot to the other writer; dropping the window and judging every finding — a judge call per finding per label, and a wrong finding at the right words credited anywhere; `also` as `path:line` strings across files — no label needs another file yet.

### Re-scoring stored runs

**ID:** regrade-from-stored-outputs
**Status:** superseded by regrade-reads-page-copy
**Chosen:** `flow-eval regrade <run-id>…` re-grades each review trial that was graded, from its `runs/<id>/<case>/<trial>/output.md`, or — when that gitignored file is gone — from the `output.md` its committed run page embeds, which it first restores to that path so the re-rendered page keeps it. The header takes the current case hashes and judge pin and a `regraded` field naming the flow-eval sha; trial metrics, cost and wall time are kept, the review grade, score, pass and judge usage replaced. Every kan-867 review run is re-graded, other slots included, so they stay comparable under the new case hash.
**Considered:** re-running the trials — costs every candidate call again and measures a different sample; re-grading only runs whose trial directories exist — leaves the committed baseline and the older runs scored by the old labels and incomparable to new runs.
**Superseded because:** task 8 made every re-rendered page keep an artifact whose trial file is gone, so writing the page copy back to `output.md` became a second copy and a write before every trial had re-graded (panel finding F9).

### Re-scoring stored runs, reading the page's copy in place

**ID:** regrade-reads-page-copy
**Status:** active
**Chosen:** as `regrade-from-stored-outputs`, except the page's copy of `output.md` is graded where it lies and never written back — the re-rendered page keeps it (task 8) — and a copy the page cut at its size cap or could not read is refused, leaving the run untouched.
**Considered:** writing the copy back to `output.md` — redundant since task 8, and it writes before every trial has re-graded; grading a cut copy — scores a truncated report as the trial's.

### A review run's id names its slot

**ID:** run-id-names-slot
**Status:** active
**Chosen:** a review run's id is `<started>-review-<slot>-<harness>-<model>-<effort>`; impl and visual ids are unchanged. Refines kan-964 design.md §8's run id. Committed runs keep the ids they were recorded under: nothing parses an id — the index lists `runs/*.jsonl` and reads each header — so old and new ids sit side by side.
**Considered:** a sub-second stamp — still collides when two launches share a stamp, and hides the slot a reader looks for in the id; renaming the committed review runs — rewrites every baseline and compare file that names them, for no reader that needs it.

### A bold-led finding keeps its bullets

**ID:** parse-bold-led-bullets
**Status:** active
**Chosen:** under a severity heading, a finding opened by a bold-led paragraph or a `#####` sub-heading runs over the list items that follow it; only the next bold-led finding, sub-heading or heading ends it, or a paragraph that is a bold label alone (`**Considered and not raised:**`), whose own items are no finding. A heading ends a paragraph as a blank line does, so a bold-led finding directly under its severity heading opens. A bold-led paragraph opens a finding when it names a path:line, as before, or when it opens with an item number (`**3. …`). Refines kan-964's report parse (design.md §7 step 1).
**Considered:** dropping list items that carry no path:line — a list-led finding with no location (`2. **Unlocated** — …`) would vanish; parsing only numbered items — every list-led report would lose its findings.

### Real extras are rewarded

**ID:** review-extras-rewarded
**Status:** superseded by review-extras-net
**Chosen:** an extra is a finding of any severity that matches no `mustFind` or `knownFalse` label and that the judge rates a plausible defect in the diff (the existing plausibility question). A review trial's score is F1 × 100 plus 5 per extra, at most 15 (three extras), so it may exceed 100; `extras` is recorded beside the other review metrics. A Minor rated implausible stays outside precision, as before; pass is still recall 1. Refines kan-964's review score (design.md §7, `F1 × 100`).
**Considered:** extras as true positives in precision — no reward once precision is already 1; an uncapped bonus — a reviewer that pads its report with plausible-sounding findings gains without bound; a hand-curated `valid` label list — rewards only extras someone has already listed, never a new one.
**Limit:** the judge sees the diff, not the running code, so "plausible" is the evidence for "real"; two findings on one extra count twice, bounded by the cap.
**Superseded because:** an unmatched Minor was upside-only — plausible earned 5, implausible cost nothing, since precision ignores Minors — so padding a report with doubtful Minors bought up to 15 points, three `compare` margins, at no risk (panel finding F13).

### Real extras are rewarded, net of implausible ones

**ID:** review-extras-net
**Status:** active
**Chosen:** as `review-extras-rewarded`, except the bonus counts net extras: plausible unlabelled findings minus implausible unlabelled findings, of every severity, floored at 0 — 5 points each, at most 15. An unlabelled finding is first put to the judge's same-defect question against every `mustFind` label, outside the ±15-line window too; one it matches is neither an extra nor implausible (F14). `extras` and `implausible` are both recorded; the score derives the net. A Critical or Important rated implausible still also counts against precision.
**Considered:** extras among Critical and Important only — no Minor real extra is ever rewarded, the case that motivated the reward; keeping extras out of the score — rewards nothing.
**Limit:** as `review-extras-rewarded`'s; an implausible Critical or Important now costs both precision and one net extra.

## Open questions

### The eval credential

**ID:** eval-credential
**Status:** answered by eval-credential-from-zshrc
**Why it is open:** neither `CLAUDE_CODE_OAUTH_TOKEN` nor `ANTHROPIC_API_KEY` is set in this session's shell, and no profile file sets them; the earlier runs had one.
**What it affects:** every run task; implementation asks the operator for it before the first run (a file path the run commands read, never the value in a command line or transcript).

## Live check

The runs are the live check: each task that changes the prompt runs the real candidate against the real case through the real judge, and a variant that scores 0/6 is the recognisable failure. No service or persistent state changes.
