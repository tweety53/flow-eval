# 20261013T120039Z-visual-claude-code-sonnet-low vs 20261013T121649Z-visual-claude-code-opus-low

- role `visual`, margin 5 points
- candidate `claude-code` `sonnet` `low` served claude-sonnet-5-5; baseline `claude-code` `opus` `low` served claude-opus-5-5
- flow-eval 3ec26bad71d9d5092cdd2feca4655c9b9c126143 vs 3ec26bad71d9d5092cdd2feca4655c9b9c126143
- warning: served model claude-sonnet-5-5 vs claude-opus-5-5 — DIFFER: the runs were served different models
- role score 100.0 vs 100.0 (+0.0)
- cost per pass $0.0812 vs $0.1372
- false-pass 0 vs 0

| case | score | base score | delta | cost | base cost | mean wall s | base mean wall s | tokens | base tokens |
|---|---|---|---|---|---|---|---|---|---|
| kan-870-qm-q6 | 100.0 | 100.0 | +0.0 | $0.2437 | $0.4117 | 10.2 | 12.5 | 159386 | 147417 |

## Flips

None.

**Verdict:** non-inferior
