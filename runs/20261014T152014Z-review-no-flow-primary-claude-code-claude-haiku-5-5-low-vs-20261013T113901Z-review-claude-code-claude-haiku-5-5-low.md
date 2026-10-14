# 20261014T152014Z-review-no-flow-primary-claude-code-claude-haiku-5-5-low vs 20261013T113901Z-review-claude-code-claude-haiku-5-5-low

- role `review`, margin 5 points
- candidate `claude-code` `claude-haiku-5-5` `low` served claude-haiku-5-5, no-flow; baseline `claude-code` `claude-haiku-5-5` `low` served claude-haiku-5-5
- flow-eval 0f04b34c54a24f84e994d9face0000f28d6713ea vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143 — DIFFER: grading may have changed between the runs
- role score 0.0 vs 35.0 (-35.0)
- cost per pass – vs $0.0264

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 0.0 | 35.0 | -35.0 | $0.0140 | $0.0264 | 19.9 | 29.8 | 175383 | 428197 |

## Flips

None.

**Verdict:** regression: role score 0.0 < 35.0 − 5
