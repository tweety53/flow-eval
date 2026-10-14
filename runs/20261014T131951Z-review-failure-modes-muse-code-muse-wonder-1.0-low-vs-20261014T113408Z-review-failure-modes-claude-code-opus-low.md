# 20261014T131951Z-review-failure-modes-muse-code-muse-wonder-1.0-low vs 20261014T113408Z-review-failure-modes-claude-code-opus-low

- role `review`, margin 5 points
- candidate `muse-code` `muse-wonder-1.0` `low` served muse-wonder-1.0; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval 677795466f6741c22a360b387ead91e1a1c61d1b vs a144e07d3bccf28243f655296c8ece49608a2cca — DIFFER: grading may have changed between the runs
- warning: prompt pin dc185c68bbcfebeb3a96e2dadd5f665a59da3dd3 vs 3934752454570b5412c2992304b0126c73d24686 — DIFFER: prompts may have changed between the runs
- warning: served model muse-wonder-1.0 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 103.3 vs 100.0 (+3.3)
- cost per pass $0.5410 vs $0.2065

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 103.3 | 100.0 | +3.3 | $1.6229 | $0.6194 | 61.7 | 28.8 | 1025331 | 207307 |

## Flips

None.

**Verdict:** non-inferior
