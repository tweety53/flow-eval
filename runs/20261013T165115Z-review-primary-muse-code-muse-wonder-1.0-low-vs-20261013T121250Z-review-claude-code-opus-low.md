# 20261013T165115Z-review-primary-muse-code-muse-wonder-1.0-low vs 20261013T121250Z-review-claude-code-opus-low

- role `review`, margin 5 points
- candidate `muse-code` `muse-wonder-1.0` `low` served muse-wonder-1.0; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval da80df7267c60fdc5835c8e4c22afcf1b3675adf vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143 — DIFFER: grading may have changed between the runs
- warning: prompt pin 2441dced49b875569f50b89391b75e2b71c7595d vs 2e053aaa95ce00195dac3b208e1769dcc45862da — DIFFER: prompts may have changed between the runs
- warning: served model muse-wonder-1.0 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 105.0 vs 101.7 (+3.3)
- cost per pass $0.7233 vs $0.2901

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-867-qm-2-step-1-frontend-settings-workout-mode-focus | 105.0 | 101.7 | +3.3 | $2.1699 | $0.8702 | 84.8 | 37.0 | 2027102 | 392547 |

## Flips

None.

**Verdict:** non-inferior
