# 20261013T163922Z-review-muse-code-muse-spark-1.3-low vs 20261013T121250Z-review-claude-code-opus-low

- role `review`, margin 5 points
- candidate `muse-code` `muse-spark-1.3` `low` served muse-spark-1.3; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval aed7d0ffcec43faaee302c8bfc9957bb0e43ef0b vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143 — DIFFER: grading may have changed between the runs
- warning: prompt pin 2441dced49b875569f50b89391b75e2b71c7595d vs 2e053aaa95ce00195dac3b208e1769dcc45862da — DIFFER: prompts may have changed between the runs
- warning: served model muse-spark-1.3 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 16.7 vs 101.7 (-85.0)
- cost per pass $0.5435 vs $0.2901

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 16.7 | 101.7 | -85.0 | $0.5435 | $0.8702 | 48.3 | 37.0 | 1130995 | 392547 |

## Flips

None.

**Verdict:** regression: role score 16.7 < 101.7 − 5
