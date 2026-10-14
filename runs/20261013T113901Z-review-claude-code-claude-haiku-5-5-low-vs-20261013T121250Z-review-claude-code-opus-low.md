# 20261013T113901Z-review-claude-code-claude-haiku-5-5-low vs 20261013T121250Z-review-claude-code-opus-low

- role `review`, margin 5 points
- candidate `claude-code` `claude-haiku-5-5` `low` served claude-haiku-5-5; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval 3ec26bad71d9d5092cdd2feca4655c9b9c126143 vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143
- warning: served model claude-haiku-5-5 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 35.0 vs 101.7 (-66.7)
- cost per pass $0.0264 vs $0.2901

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 35.0 | 101.7 | -66.7 | $0.0264 | $0.8702 | 29.8 | 37.0 | 428197 | 392547 |

## Flips

None.

**Verdict:** regression: role score 35.0 < 101.7 − 5
