# Self-review: kan-964-flow-eval-harness

**Deferred:** reasoning pass run on Opus 5.5 from the in-run bundle; the pass covers what the bundle holds and nothing beyond it

## Problems encountered, and what pipeline change would avoid them — `flow-fix`

- **[flow-fix]** A fix-round re-review on sonnet/low reported "all fixed" without reading a hunk (round 1; F17 surfaced only on send-back). Re-review verdicts now cite the fix-diff hunk, checked by `check-rerun-citations.sh`, one `-cite` send-back per failing re-run — fixed: 7ae95a5b

## Token/time cost, and what would reduce it without quality loss — `flow-cost`

- **[flow-cost]** 58 of 119 ledger rows read `Tokens: not measured`; 36 traced to the harvester failing to pair `dispatch begin` commands written in `;`-chains, with `$VAR` session tokens, same-instant ties and inline begins (22 are inline by design) — fixed: 4dcc2bc5
- **[flow-cost]** The harvester counted a message twice when its JSONL lines straddled two reads, inflating every measured row (e.g. task-1-implementer 109 vs 67 messages) — fixed: b77bb687
- **[flow-cost]** 27 panel rounds over 6 fix runs: every appended task got its own late-fix round plus post-fix re-reviews; batching needs a design choice — declined

## What went well, and how to reproduce it — `flow-improvement`

_none — this angle produced no findings._

## What could be automated or moved to a script — `flow-automation`

_none — this angle produced no findings._

## What could move to the Go app or its persistent storage — `flow-stats-app`

- **[flow-stats-app]** `flow record finding` accepted sha-shaped `-ref`s (two mis-keyed rows inflated the panel to 65 findings); it now refuses any ref not shaped `F<n>` — fixed: 7a9e488b
- **[flow-stats-app]** kan-964's existing store rows repaired in the local store at the operator's instruction: 10 misattributed and 36 empty agent ids re-paired and their tokens recounted from transcripts (unmeasured 58 → 22, all inline); snapshot kept before the write — fixed: data repair, no commit

## What can be sped up — `flow-speed`

_none — this angle produced no findings._

**Rating:** 3 — fine
