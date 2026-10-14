# Self-review: kan-978-tune-principles-prompt

**Deferred:** reasoning pass run on Opus 5.5 from the in-run bundle; the pass covers what the bundle holds and nothing beyond it

## Problems encountered, and what pipeline change would avoid them — `flow-fix`

- **[flow-fix]** `/flow <name>` at `IN_PROGRESS` read literally as a fix run (router: "an argument present"), though every handoff ends `Next: /flow <name>`; an argument exactly naming a candidate change is now a bare invocation — fixed: cc76eb5f
- **[flow-fix]** `close-task.sh` accepted task 2's agents commit `0ff796c0` with no `Task-Id:` trailer because its sha was passed explicitly; close-task now refuses a task commit whose trailer does not name the task — fixed: 10044ea3
- **[flow-fix]** `check-task-commit-fields.sh` read backticked id suffixes in a `**Tests:**` field as test names; traced to the documented contract (every backticked token there is a test name) — noted
- **[flow-fix]** Task 3's `**Commit:**` carried a `<short sha>` placeholder that plan-shape accepted and the commit guard refused; angle brackets are legal subject text (11 subjects on agents main), so plan-shape cannot refuse them — noted
- **[flow-fix]** The plan did not name agents' `verbatim-moves.txt` for the prompt's new sentence; `check-verbatim-moves.sh` caught it and planning commits added it — noted
- **[flow-fix]** Fix run 2's inline plan-append failed its `tasks.md` anchor after writing `proposal.md` and `design.md`, so the planning landed in two commits — noted

## Token/time cost, and what would reduce it without quality loss — `flow-cost`

- **[flow-cost]** Panel-fix 5 ran the 21-run re-grade inside the fix subagent and hit a usage limit nine runs in; the next session finished the rest — noted

## What went well, and how to reproduce it — `flow-improvement`

- **[flow-improvement]** The plan's ≥3/6 stop rule ended prompt tuning after variant 1 (0/6 → 3/6 on haiku-low), saving two k=6 runs — noted

## What could be automated or moved to a script — `flow-automation`

- **[flow-automation]** The runner rebuild after every `prompts sync` is a copy-pasted multi-command line in the plan's Global constraints — noted

## What could move to the Go app or its persistent storage — `flow-stats-app`

- **[flow-stats-app]** Four dispatches (task-4-implementer, full-suite-fix-1, task-12-reviewer-retry and others) carry no token figure — noted

## What can be sped up — `flow-speed`

- **[flow-speed]** Reproducers 5-primary-1 and 5-primary-3 pinned pre-fix lines as premises, so their post-fix re-runs were refused and had to be re-authored — noted

**Rating:** 4 — good
