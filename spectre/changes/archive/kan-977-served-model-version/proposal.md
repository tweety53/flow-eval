# kan-977-served-model-version

KAN-977.

## Why

- A run records only the requested model alias (`sonnet`, `opus`) at run level. The alias moves when a new model ships, so two `sonnet` runs a month apart can be different models, and nothing on the index, the run page or `compare` says so.
- The concrete id the trials were served (`claude-sonnet-5-5`) is already in every trial line as `served_model`, but no run-level view shows it.

## What changes

- Every run shows its **served model** — the distinct `served_model` ids of its trials — in `runs/<id>.md`, `runs/<id>.html` and a new `served` column of `runs/index.html`; existing runs show it on their page and in the index once repainted, since it is derived from their trial lines (their `.md`, written once by `run`, is not rewritten).
- `compare` names both sides' served models and warns `DIFFER` when they differ; the warning never changes the verdict or the exit.
- A harness that reports no served model (codex) shows `–` and never warns.
