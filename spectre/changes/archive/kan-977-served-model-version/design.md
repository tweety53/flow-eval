## Context

- Run-level views (`runs/<id>.md`, `runs/<id>.html`, `runs/index.html`, `compare`'s report) name only the requested model alias; the served id lives per trial in `served_model`. One change: the same derived value feeds every view.

## Served model

- `run.Served(lines []Line) string`: every trial line's `served_model` split on `,`, empties dropped, de-duplicated, sorted, joined with `, `; `""` when no line names one. Infra-error lines count too: a model that answered was served.
- `summary` gains `Served`, set by `summarize`, so the `.md` report, the page and the index read one value.
- Rendering: `.md` settings line gains `, served <ids>` (or `–`); the page header gains `<dt>served</dt>` after `model`; the index gains a `served` column after `model`. `""` renders `–`.
- `compare`: the report's candidate/baseline line names each side's served set; `Decision.Served` is the warning `warning: served model <cand> vs <base> — DIFFER: the runs were served different models` when both sets are non-empty and differ, else `""`. `Warnings()` prints it after the judge warning. Never refuses, never moves the exit.

## Decisions

### Derive the served model from trial lines

**ID:** served-derived-not-stored
**Status:** active
**Chosen:** derive from the trial lines on every read — existing runs get it with no migration, and the header keeps holding only what is known when the run starts.
**Considered:** a `served_model` field in the header — the header is the first line, written before any trial runs, so it would need the `.jsonl` rewritten at the end, and every run from before the field would show nothing.

### Model version means the served model id

**ID:** version-is-served-id
**Status:** active
**Chosen:** the served model id(s) — the operator's answer; it is what the alias resolved to and what pricing already keys on.
**Considered:** the harness CLI version as well, or alone — out of scope by the operator's choice; it says which client ran the model, not which model answered.

### Warn only when both sides name a served model

**ID:** served-warning-both-known
**Status:** active
**Chosen:** warn when both sets are non-empty and differ — an unknown side (codex) is not evidence of a change.
**Considered:** warn when either side is unknown, as the judge warning does for an unrecorded judge — every codex comparison would carry a warning nobody can act on.

## Open questions

## Live check

- The muse-wonder-1.0 runs that follow this change: `runs/index.html`'s `served` column and each new run page read `muse-wonder-1.0`; each pre-existing claude-code run reads `claude-haiku-5-5` or `claude-sonnet-5-5` after the repaint the new runs trigger.
- Not working looks like `–` in the `served` column of a muse or claude-code run, or no `served` column at all.
