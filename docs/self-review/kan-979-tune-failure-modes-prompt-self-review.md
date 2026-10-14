# Self-review: kan-979-tune-failure-modes-prompt

**Deferred:** reasoning pass run on Opus 5.5 from the in-run bundle; the pass covers what the bundle holds and nothing beyond it

## Problems encountered, and what pipeline change would avoid them — `flow-fix`

- **[flow-fix]** `render-slot-prompt.sh` could not render an `exp-` slot; fixed in-run on agents main — fixed: 5db62f41
- **[flow-fix]** The narrative's `In-run pipeline fix:` line carried no sha and no blast radius, off the contract's format, so the pass could not read the sha off the bundle; `integrate-change.sh commit` now refuses an off-format line with `STOP: narrative` — fixed: aceb687e
- **[flow-fix]** `flow record summary` is called nowhere, so every bundle's first source reads `skipped: change summary (absent)`; the summary contract now records each printed summary — fixed: 937955e9
- **[flow-fix]** Fix run 1's dispatches, pass rows and findings never reached the store until the pass noticed; `check-unfinished-work` now reports a ticked appended task with no dispatch row as unfinished work — fixed: 0591e34b
- **[flow-fix]** The landed agents winner commit lacked its `Task-Id:` trailer, forcing a force-push; `docs/prompt-tuning.md` § Land step 1 now requires it before the push — fixed: ba382b6
- **[flow-fix]** A stray `.DS_Store` nearly entered the reshaped commit; `.gitignore` now ignores it — fixed: 64a8a51
- **[flow-fix]** A PreToolUse hook blocked a heredoc planning append naming the main checkout's path; not reproduced, the exact command is not in the bundle — noted

## Token/time cost, and what would reduce it without quality loss — `flow-cost`

- **[flow-cost]** Most of what the panel read was generated run records (~2.2M cache read per reviewer), and committing `runs/<id>/` adds ~60k lines more; `.gitattributes` marks transcripts, run pages and `.jsonl` `-diff`, keeping compare reports diffable — fixed: f48ac5a

## What went well, and how to reproduce it — `flow-improvement`

- **[flow-improvement]** The in-run pipeline fix ran in parallel with the panel and landed on agents main while the run went on — noted
- **[flow-improvement]** The mutation slot proved a real test gap (the render test now pins the failure-recovery step), and every withdrawn Minor recorded its code-path reason — noted
- **[flow-improvement]** The early-stop rule held: variant 2 scored 4/6, so variant 3 was skipped — noted

## What could be automated or moved to a script — `flow-automation`

- **[flow-automation]** The post-landing `prompts/PIN` re-pin was a recurring hand-written commit; § Land step 5 is now one command that finds the landed sha by exact subject, syncs and fails on changed prompt bytes — fixed: c812552

## What could move to the Go app or its persistent storage — `flow-stats-app`

- **[flow-stats-app]** The plan derived passes with `jq` over the run's `.jsonl` though the run's `.md` summary already carries a `passes` column — planner redundancy, not a pipeline defect — noted

## What can be sped up — `flow-speed`

None — the panel slots, the in-run fix and the verifiers already overlapped, and eval runs are serial by design.

**Rating:** 4 — good
