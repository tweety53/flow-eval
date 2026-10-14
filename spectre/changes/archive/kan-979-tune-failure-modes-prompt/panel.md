# Review panel — kan-979-tune-failure-modes-prompt

Rendered from the store. Do not edit: the findings are rows, and the next render overwrites this file.

| ID | Slot | Severity | Location | Note | Lineage |
|---|---|---|---|---|---|
| F1 | primary | Minor | runs/20261013T114128Z-review-claude-code-claude-haiku-5-5-low-vs-20261013T121524Z-review-claude-code-opus-low.md:1 | task 6 deleted the runs but left six tracked compare reports naming a deleted candidate (three also a deleted baseline). |   |
| F2 | primary | Minor | docs/prompt-tuning.md:135 | the worked-example row cites agents 39347524, the pre-rebase sha, which no ref keeps once the change branch lands and is deleted. |   |
| F3 | mutation | Minor | prompts/review-failure-modes.md:37 | deleting the failure-recovery step from the vendored prompt fails no test or guard. |   |
| F4 | mutation | Minor | prompts/PIN:1 | reverting the PIN to the old sha while keeping the new prompt fails no test. |   |
| F5 | mutation | Minor | baselines/review-failure-modes.json:1 | pointing the baseline at a deleted run fails no test. |   |
| F6 | mutation | Minor | runs/index.html:56 | restoring the old index that lists the deleted runs fails no test. |   |
| F7 | mutation | Minor | runs/20261013T120012Z-review-claude-code-sonnet-low-vs-20261013T121524Z-review-claude-code-opus-low.md:1 | compare reports name runs task 6 deleted; step 2 removed .jsonl/.md/.html but not these reports. |   |

findings-total: 7
finding-status: F1 fixed
finding-status: F2 fixed
finding-status: F3 fixed
finding-status: F4 withdrawn prompts sync writes PIN and the vendored prompts together from one agents sha, so a PIN at odds with the prompt is a hand edit no code path makes, and land step 2 re-syncs and diffs the bytes
finding-status: F5 withdrawn baseline set loads the run before writing the baseline file, so no command can point a baseline at a missing run; a hand-edited json is not a code path
finding-status: F6 withdrawn runs/index.html is generated output that every run and baseline set rewrites from the run files; restoring a stale copy by hand is not a code path
finding-status: F7 fixed

reproducers-total: 7
finding-reproducer: F1 .superpowers/sdd/reproducers/0-primary-1.sh
finding-reproducer: F2 .superpowers/sdd/reproducers/0-primary-2.sh
finding-reproducer: F3 .superpowers/sdd/reproducers/0-M1-1.sh
finding-reproducer: F4 .superpowers/sdd/reproducers/0-M2-1.sh
finding-reproducer: F5 .superpowers/sdd/reproducers/0-M3-1.sh
finding-reproducer: F6 .superpowers/sdd/reproducers/0-M4-1.sh
finding-reproducer: F7 .superpowers/sdd/reproducers/0-I1-1.sh

## Pass log

### Round 0

- roster: full
- diff size: flow-eval 7883 (over cap), agents 50 (under cap), sum 7933 — proceeded automatically; the bulk is committed eval run records
- docs-only: exit 1 both worktrees (flow-eval baselines/review-failure-modes.json, agents spectre/changes/kan-979-tune-failure-modes-prompt/verbatim-moves.txt); roster dispatched: primary+principles+failure-modes, mutation+exp-test-sanity
- no addition this round — the resolved list ran alone; standards: none declared; citation check: agents scripts/check-references.sh captured
- mutation+exp-test-sanity: exp-test-sanity brief typed beside the rendered mutation brief — render-slot-prompt.sh cannot render an exp- slot (in-run pipeline fix pipeline-fix-1 dispatched)
