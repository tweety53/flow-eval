# 20261014T132529Z-review-failure-modes-muse-code-muse-spark-1.3-low vs 20261014T113408Z-review-failure-modes-claude-code-opus-low

- role `review`, margin 5 points
- candidate `muse-code` `muse-spark-1.3` `low` served muse-spark-1.3; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval 677795466f6741c22a360b387ead91e1a1c61d1b vs a144e07d3bccf28243f655296c8ece49608a2cca — DIFFER: grading may have changed between the runs
- warning: prompt pin dc185c68bbcfebeb3a96e2dadd5f665a59da3dd3 vs 3934752454570b5412c2992304b0126c73d24686 — DIFFER: prompts may have changed between the runs
- warning: served model muse-spark-1.3 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 77.8 vs 100.0 (-22.2)
- cost per pass $0.1892 vs $0.2065

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 77.8 | 100.0 | -22.2 | $0.5677 | $0.6194 | 45.7 | 28.8 | 1087203 | 207307 |

## Flips

None.

**Verdict:** regression: role score 77.8 < 100.0 − 5
