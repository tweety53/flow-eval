The capture does not match the mockup. I compared the two images by eye only: the band and seam measurement didn't run, because the output parser I piped it into (`python3`) isn't installed. The differences below are clear at full-page scale, so the measurements wouldn't change the verdict. A control-level matrix was not done.

- defect: The top bar in the capture has a grey fill and a hairline under it. The mockup's header sits on the page background with no rule.
- defect: The back button in the capture is an arrow (←). The mockup draws a chevron (‹).
- defect: The date line reads "Fri 9 Oct" in the capture. The mockup shows "Fri 9 Oct · 18:30", so the time is missing.
- defect: The work step reads only "800 m" in the capture. The mockup's "@ 4:00–4:10 /km" pace target is missing.
- defect: The "last time" summary in the capture shows only "31:40". The mockup shows "4.9 km · 31:40 · 6:28 /km · 152 bpm · effort 7", so distance, pace, heart rate and effort are missing.
- defect: Inside the repeat card, the capture indents the "6×" header and the work/recover rows (with their dividers) further right than the mockup. In the mockup "6×" sits at about x=64 and the rows at about x=88; in the capture they're at about x=88 and x=136.
- defect: The hairline under the repeat card (above "Cool-down") is missing in the capture.
- defect: Vertical spacing is larger throughout the capture. "Planned", the stats, the warm-up, the card and its rows sit lower, and the card runs from about y=655–1020 against about y=575–890 in the mockup.
- defect: The DISTANCE stat column starts at about x=325 in the capture, against about x=280 in the mockup, so the gap between the two stats is wider.
- defect: The section-header rules beside "OUTDOOR RUN · INTERVALS" and "LAST TIME · TUE 29 SEP" start at different x positions. The kicker text is also wider-tracked in the capture.
- defect: The bottom action bar's divider and buttons sit at different heights: the buttons are about 1488–1584 in the capture against 1496–1584 in the mockup.

The "Start → P2 / Log result → L1" chips in the mockup are frame annotations, not app content, so their absence from the capture is not a defect.

verdict: mismatch