# 20261013T114334Z-impl-claude-code-sonnet-low vs 20261013T120126Z-impl-claude-code-opus-low

- role `impl`, margin 5 points
- candidate `claude-code` `sonnet` `low` served claude-sonnet-5-5; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval 3ec26bad71d9d5092cdd2feca4655c9b9c126143 vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143
- warning: served model claude-sonnet-5-5 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 100.0 vs 100.0 (+0.0)
- cost per pass $0.4058 vs $0.9119

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-757-offline-l5-be-per-item-if-match-on-group-session-task-3 | 100.0 | 100.0 | +0.0 | $1.2175 | $2.7357 | 644.0 | 544.5 | 2228496 | 2709592 |

## Flips

None.

**Verdict:** non-inferior
