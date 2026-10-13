# Review panel — kan-977-served-model-version

Rendered from the store. Do not edit: the findings are rows, and the next render overwrites this file.

| ID | Slot | Severity | Location | Note | Lineage |
|---|---|---|---|---|---|
| F1 | primary | Minor | spectre/changes/kan-977-served-model-version/proposal.md:12 | The proposal says existing runs show the served model in runs/<id>.md too, but runs/<id>.md is written only once, by Run; Repaint rewrites pages and the index, never the .md. |   |
| F2 | principles | Minor | internal/compare/compare.go:174 | The warning always says "the model behind the alias may have changed", but compare accepts runs that requested different models, so a haiku-vs-sonnet compare blames an alias change. |   |

findings-total: 2
finding-status: F1 fixed
finding-status: F2 fixed

reproducers-total: 2
finding-reproducer: F1 .superpowers/sdd/reproducers/0-primary-1.sh
finding-reproducer: F2 .superpowers/sdd/reproducers/0-principles-1.sh

## Pass log

### Round 0

- roster: compact — 84
- diff size 294 lines, under cap; docs-only exit 1 (internal/compare/compare_test.go) — roster primary+principles dispatched; no addition this round — the resolved list ran alone; standards: none declared
