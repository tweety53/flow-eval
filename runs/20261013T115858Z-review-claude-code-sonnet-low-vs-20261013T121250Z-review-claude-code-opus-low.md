# 20261013T115858Z-review-claude-code-sonnet-low vs 20261013T121250Z-review-claude-code-opus-low

- role `review`, margin 5 points
- candidate `claude-code` `sonnet` `low` served claude-sonnet-5-5; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval 3ec26bad71d9d5092cdd2feca4655c9b9c126143 vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143
- warning: served model claude-sonnet-5-5 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 0.0 vs 61.1 (-61.1)
- cost per pass – vs $0.2901

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 0.0 | 61.1 | -61.1 | $0.4020 | $0.8702 | 30.1 | 37.0 | 342868 | 392547 |

## Flips

- kan-867-qm-2-step-1-frontend-settings-workout-mode-focus: pass^k → 0

**Verdict:** regression: role score 0.0 < 61.1 − 5; kan-867-qm-2-step-1-frontend-settings-workout-mode-focus: pass^k → 0
