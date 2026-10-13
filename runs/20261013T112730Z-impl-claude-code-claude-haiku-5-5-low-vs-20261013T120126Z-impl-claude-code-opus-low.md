# 20261013T112730Z-impl-claude-code-claude-haiku-5-5-low vs 20261013T120126Z-impl-claude-code-opus-low

- role `impl`, margin 5 points
- candidate `claude-code` `claude-haiku-5-5` `low` served claude-haiku-5-5; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval 3ec26bad71d9d5092cdd2feca4655c9b9c126143 vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143
- warning: served model claude-haiku-5-5 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 100.0 vs 100.0 (+0.0)
- cost per pass $0.0495 vs $0.9119

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-757-offline-l5-be-per-item-if-match-on-group-session-task-3 | 100.0 | 100.0 | +0.0 | $0.1485 | $2.7357 | 539.8 | 544.5 | 7303579 | 2709592 |

## Flips

None.

**Verdict:** non-inferior
