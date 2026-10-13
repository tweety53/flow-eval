# kan-977-served-model-version — live verification

2026-10-13, branch tip after the panel's Minor fixes.

- **Pre-existing claude-code runs:** a scratch copy of the project's `runs/` (10 runs) repainted by `flow-eval baseline set <impl run> --runs <copy>`, built from this branch.
  - before: no `served` column on `runs/index.html`, no `served` on any run page.
  - after: `index.html` carries `<th>served</th>`; the 5 runs requested `claude-haiku-5-5` read `claude-haiku-5-5`, the 5 requested `sonnet` read `claude-sonnet-5-5`, on the index and on each page. No `–`. Matched.
- **muse-wonder-1.0 runs:** not exercised — they need a real model and Docker; the first such run after merge is the check (`served` reads `muse-wonder-1.0`, else the failure look is `–`).
