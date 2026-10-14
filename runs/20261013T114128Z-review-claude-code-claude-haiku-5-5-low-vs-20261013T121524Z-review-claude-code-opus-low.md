# 20261013T114128Z-review-claude-code-claude-haiku-5-5-low vs 20261013T121524Z-review-claude-code-opus-low

- role `review`, margin 5 points
- candidate `claude-code` `claude-haiku-5-5` `low` served claude-haiku-5-5; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval 3ec26bad71d9d5092cdd2feca4655c9b9c126143 vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143
- warning: served model claude-haiku-5-5 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 1.7 vs 38.3 (-36.7)
- cost per pass – vs $0.7642

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 1.7 | 38.3 | -36.7 | $0.0213 | $0.7642 | 25.2 | 33.1 | 278818 | 271830 |

## Flips

None.

**Verdict:** regression: role score 1.7 < 38.3 − 5
