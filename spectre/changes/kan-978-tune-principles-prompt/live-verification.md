# kan-978-tune-principles-prompt — live check

The runs are the live check (design.md `## Live check`): the real candidate against the real case through the real judge. Failure look: a variant at 0/6.

- before — `runs/20261013T142803Z-review-claude-code-claude-haiku-5-5-low`, agents `2e053aaa`: finds 0/6
- variant 1 — `runs/20261013T143107Z-review-claude-code-claude-haiku-5-5-low`, agents `df6cae35`: finds 3/6 — matched
- the landed pin `2441dced` vendors byte-identical `prompts/review-principles.md` (`cmp` after `prompts sync`), so no re-run.
