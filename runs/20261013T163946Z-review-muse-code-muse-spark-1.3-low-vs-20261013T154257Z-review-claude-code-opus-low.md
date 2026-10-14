# 20261013T163946Z-review-muse-code-muse-spark-1.3-low vs 20261013T154257Z-review-claude-code-opus-low

- role `review`, margin 5 points
- candidate `muse-code` `muse-spark-1.3` `low` served muse-spark-1.3; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval aed7d0ffcec43faaee302c8bfc9957bb0e43ef0b vs b4c02a4312295b07452f705804b785489a1aba7a — DIFFER: grading may have changed between the runs
- warning: served model muse-spark-1.3 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 35.0 vs 96.9 (-61.9)
- cost per pass $0.4054 vs $0.1459

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 35.0 | 96.9 | -61.9 | $0.8108 | $0.8753 | 32.6 | 30.2 | 1185222 | 502872 |

## Flips

None.

**Verdict:** regression: role score 35.0 < 96.9 − 5
