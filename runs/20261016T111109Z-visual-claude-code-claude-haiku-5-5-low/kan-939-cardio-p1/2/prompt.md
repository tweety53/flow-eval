   **The difference panel cannot show a structural departure, so the first read of every pair
   is the script's band pairing, never the panel.** The panel is white wherever any channel
   differs, and on a real pair 20–50% of it is white from seed data, font rasterisation and the
   frame's own annotations alone. A 1px rule the capture omits, a row padding it lacks, a surface fill
   swapped for the page colour, a button border in the wrong colour: each moves the ratio by
   under a hundredth and vanishes in that white. Beside every composite the compose step writes
   the cropped frame as `<composite stem>.frame.png`, the capture's own size; for every pair, run

   ```bash
   measure-visual-properties.sh <changeRoot>/visual-verification/<stem>.frame.png <capture PNG> --scale 1 --props bands,seams
   ```

   and read `delta.bands` (the script's header is canonical for the property): every band the
   frame draws, paired with the capture's in order, a text or data difference never unpairing
   one. **Every `missing` band, every `extra` band, and every pair whose `since_pair`,
   `gap_above` or `height` delta exceeds 2px or whose `edge` or `colour` distance exceeds 12 is
   a departure until a cause is named that a band cannot carry** — a wrapped label (a `height`
   delta on a text band, confirmed by transcription), a frame annotation outside the app's own
   ink; an empty-versus-populated state is precondition 2 below, a spec to fix, never a cause to
   accept. Data never creates or removes a band, moves a divider or recolours a border, so "the
   seed data differs" explains none of them. The pairing resyncs after an unpaired band; a pair
   count far below the frame's band count is itself the finding that the layout differs
   wholesale, and the composite is then read to say how.

   **A band is one row of the page's structure and sees nothing across it, so the same call's
   seam pairing is the second read: `delta.seams`, the vertical structure inside every boxed
   band.** A segmented control with its two cell dividers gone and its wrapped labels
   left-anchored where the frame centres them is one band in both images, the same height and
   the same border colour, and pairs clean. For every boxed
   band — a bordered or filled control, a card, a hairline; never a bare text row — the script
   lists its seams (an outer border's side, a cell divider, a filled cell) paired in order, and
   for every cell between two paired seams its text lines with their `offset` from the cell's
   centre and their `left` and `right` insets. **Every `missing` seam, every `extra` seam, every
   seam pair whose `since_pair` or `width` delta exceeds 2px or whose `colour` distance exceeds
   12, every cell whose line `count` differs, and every line whose `offset` delta exceeds 2px
   is a departure until a cause is named that a seam or a line cannot carry.** Text width never
   moves a seam, so a data difference explains no seam; a line's width is data, so an `offset`
   delta whose `left` delta is within 2px (or whose `right` is) is a left- (right-) anchored
   label whose text differs — the cause is then the transcription, and it is named — where an
   `offset` delta with `left` and `right` both moved by the same amount is a label anchored to a
   different edge than the frame's, a departure whatever the text reads. What the seams cannot
   see — a divider in a row with no border or fill, the label of a filled cell, a glyph, a
   corner radius — the per-control sweeps below still measure.

   **A full-page match — a clean composite read, a structural match — is necessary, never
   sufficient: verify at the control level, measure rather than eyeball, and exercise the states
   a resting frame does not show.** Full-page comparison catches wrong regions and wrong overall
   layout, and wrong text only where the capture's data is the frame's own; it does not catch a
   border style, an icon's glyph, or a colour step, all of
   which are invisible at full-page scale. Before
   accepting any field, button, icon, or toggle as matching its mockup:
   1. **Crop and zoom (2–3x) both the mockup region and the corresponding capture, side by side.**
      Do this for every interactive control the view carries, not only ones that already look
      suspicious at full scale. **Name the control's kind in each image first, before any
      measurement** — a plain text field or a headline display figure; a list row carrying a
      leading radio, or a quiet row whose only indicator is a trailing filled circle on the
      selected one; a segmented control of N cells each fitting its own label — a differing
      kind is a departure before any number is taken.
   5. **Every text run in the capture, transcribed and judged as display text.** The difference
      panel is white wherever the capture's data differs from the frame's, and "the data differs"
      is the explanation that absorbs a wrong number: a white label region is not accounted for
      until its text has been read. Transcribe every number, unit and label the capture shows and
      state, per run, that it is a display value at the precision the frame shows — a raw float,
      a `NaN`, `null`, `undefined`, an empty string where the frame draws a value, a placeholder
      is a defect whatever the frame's own numbers are.
   6. **The elements of capture and frame, listed top to bottom, and every element inside its
      own container.** Write both lists — heading, chart, chips, list, link — and compare their
      order: a control present in both but in a different position relative to its siblings is a
      departure the per-control crop never sees, since each crop matches its own control wherever
      it sits. Then
      confirm no element's ink crosses its container's bounds into a sibling — a plotted line, a
      marker, a label.
   9. **Every row the frame bounds with a divider, hairline or container edge — a header bar, a
      sticky bar, a toolbar, a list section, a dialog's action row — gap-measured on the side
      facing each bound, in both images, before the row is called matching.** Which rows are
      bounded is read from the frame, never from the capture: a capture that omits the frame's
      hairline shows no bounded row to measure, and the band pairing above is what finds the
      omission. The top and bottom gaps are the row band's `gap_above` and its successor's in
      the band pairing above, already paired per image; the left and right gaps are read from
      the `gap` block of the
      per-control measurement below for the row's controls: the script scans past the region to
      the next neighbour, so the number is already in
      the JSON of every measurement this step makes and costs nothing beyond reading it. This
      sweep runs on every such row, unprompted: the spacing rule below answers a claim or a
      complaint, and a row nobody claimed anything about was never measured. A row
      whose every control measures the right size is the case to suspect, not to pass: a uniform
      container padding around correctly-sized controls is invisible to every reading but this one.
   10. **Every non-text ink the frame draws — hairline, divider, rule, border, background fill,
      shadow band — inventoried from the frame alone, then found in the capture one by one.**
      The horizontal inks — every hairline, rule, fill band and border edge the frame draws across
      the page — are the frame's bands, and the band pairing above has already listed each one
      `missing` or paired with its colour; the vertical inks inside every bordered or filled
      band — a control's sides, its cell dividers, a card's inner rules — are its seams, and the
      seam pairing above has listed each one the same way; this sweep reads both lists and adds
      the vertical inks neither can see, the dividers of a row with no border or fill: run
      `measure-visual-properties.sh` with `--props runs` on the
      frame alone, the region a full-width row through the centre of each such row, write down
      every run whose colour is neither the
      background's nor a text run's — its position and length — then the same region on the
      capture, and pair the lists: a run on the frame's list with no counterpart on the
      capture's is a departure, whatever the row's text reads. This sweep runs on a frame whose
      content already matches and on a frame
      already fixed for something else — a fix run re-verifies the whole frame, never the
      element it fixed.

   **Every element the frame draws, on every property the script measures — a matrix per
   frame, never a list of sweeps done.** Every sweep above is anchored on one kind of element
   and asks the question its own incident taught: a control's kind and box, a text run's
   content, a bounded row's gaps, a column's ink. An element outside every anchor — a headline
   figure, a caption, a summary row, a slider's track and thumb, a row's own border — is
   transcribed at most, and a property no sweep names is read by nobody: the text sweep judges
   what a run says and never its colour or its size, the row sweep measures the gaps around a
   row and never how far the row stretches, and the per-control crop is asked for the one number
   the last incident made memorable. The report's `sweeps:` line then names the sweeps, not the
   numbers, so a round that re-checked one defect reads exactly like a round that measured the
   frame. Before a frame is called matching:

   1. **Enumerate the elements from the frame, not the capture and not the last report** —
      every visually distinct thing it draws, top to bottom: each text run (a heading, a figure,
      a label, a caption, a unit), each control (a field, a button, a segment, a slider's track
      and its thumb separately, an icon), each row, card or section container, each rule, divider
      and fill band. An element in the capture the frame does not draw is itself a row, marked
      `extra`. A fix run enumerates afresh from the frame — the previous round's departures are
      cells in the new matrix, never its rows.
   2. **Fill every column for every row from the script's own output**, one call per element:

      | column | text run (`--props ink`, the crop spanning the run's container's full width) | control, row, card (`--props box,radius,border,fill,shadow,content,gap`) | rule, divider, fill band (`--props runs`, the region crossing it) |
      |---|---|---|---|
      | kind | the run's role (heading, figure, caption…) | the control's kind, per sweep 1 | rule / fill band |
      | content | the transcription, per sweep 5 | `content` size and `content.colour` | — |
      | colour | `ink.colour` | `fill.colour`, `border.colour`, `content.colour` | the run's `colour` |
      | size | `ink.height` — the cap-height, the font-size stand-in — and `ink.width` | `box.width` × `box.height` | the run's `length` — a rule's thickness |
      | position | `ink.left`, `ink.top` in the container crop | `gap` on all four sides; `box.left`/`box.top` | the run's `from` |
      | stretch | `ink.left` and the container width minus `ink.left + ink.width` — the insets from each container edge | `gap.left` and `gap.right` to the image edge, or to the neighbour, and `box.width` against the container's | the run's extent on the crossing scan line |
      | alignment | the two insets above, compared: equal is centred; for a line inside a boxed band's cell, its `offset`, `left` and `right` from the seam pairing | `content.padding`, all four sides | — |
      | border | — | `border` per side and its colour | — |
      | fill | — | `fill.colour` and `share` | the run's `colour` |
      | radius | — | `radius` per corner | — |
      | shadow | — | `shadow` per side | — |

      Every cell holds the frame's number, the capture's and the `delta` — or n/a — `<why>`: the
      column does not apply to the row's class (the dashes above), or the script exited 1 on a
      region no widening resolved, with the region tried. A cell left empty, or a cell filled by
      eye, is the sweep-name report again. The matrix rides `visual-verification.md` (step 11)
      under the frame's entry, and the report's `matrix:` line counts its rows and its n/a
      cells; the parent opens the frame beside the matrix and a visible element with no row, or
      an empty cell, blocks as a departure would.

   **Never judge a size, alignment, spacing, corner radius, border, fill, shadow, icon size or
   font size by eye from a resized or cropped image; measure it with
   `measure-visual-properties.sh`, then eyeball what it measured.** A crop is for reading text
   and layout, never edges or centres — interpolation and a small viewport shift an edge by
   pixels and hide a gap outright. For every per-control comparison this step makes, run

   ```bash
   measure-visual-properties.sh <mockup frame PNG> <capture PNG> --region-a x,y,w,h --region-b x,y,w,h --scale <n>
   ```

   with each region a crop around that one control and a background margin on every side —
   `--props runs` on that crop first where its edges are in doubt, since every run boundary it
   lists is an edge the region can sit on, and a miscrop is the glance again; and `--edge`
   lowered below the distance `runs` reports between a control's fill and its background where
   that distance is under the default 24, since a surface-on-page card has no hard edge at the default and exits 1 on every box property
   until it is — and
   read the JSON's `delta` block: `abs` and `pct` per numeric property, RGB distance per colour
   (the script's own header is canonical for its options, properties, output
   and exit codes). Two readings the eye reliably gets wrong: an icon's tint is `content.colour`,
   never `fill` — the fill is the box behind the glyph, and a grey glyph on the right fill matches
   on every other property; and a pill
   is a `radius` equal to half the box height, while a rounded rectangle is any smaller number —
   read the number, since both look "rounded" at 1x. **Both halves are mandatory and neither substitutes for the other.** The
   script's numbers are the only admissible measurement — no ad-hoc PIL, no reading a coordinate
   off a crop; and its output is then eyeballed against the two crops before any number is
   reported: state, per control, that the `box` the script found is that control (its edges land
   where the control's edges are seen), that `content` is the icon or label and not a corner
   artefact, and that a `null` or an exit 1 was resolved by a wider or better-centred region, not
   by dropping the property. A number no eye confirmed is a methodology error waiting to ship.
   Exit 2 with no calibration is the script refusing
   precondition 1 below; supply it, never work around it. Four preconditions on any number
   compared across two images:
   1. **Calibrate before comparing images of different provenance** — a mockup export against a
      capture, or captures at two viewport sizes. Where the project declares a `mockup frame`
      geometry, its `scale` is the calibration the composite already crops by, and the frame the
      composite cropped is at the capture's own scale, so `--scale 1`; ad-hoc measurements use
      the same factor. Otherwise derive the factor from an element whose correctness in both
      images is already established by other evidence — a prior finding, a code-level guarantee,
      a passing test — as that element's box in each image via `--ref-a`/`--ref-b`, never from
      the nearest similar-looking thing: an unchecked ruler is itself a claim, and a wrong one
      makes the comparison wrong twice.
   2. **Confirm both images are the same state of the view** — the same expand/collapse state,
      scroll position and populated/empty condition, not merely the same screen. The box between
      two landmarks encloses different content in different states, and its size then compares
      nothing.
   3. **Treat an implausible result — a multiple rather than a percentage, an order of
      magnitude — as a methodology error, never as a bigger finding.** Re-derive the calibration
      and the state check before reporting it; a smaller wrong number from the same mistake reads
      as a finding and ships.
   4. **Measure the rendered box, never the declared one — a declared minimum is a floor, not a
      size, and the hit box and the visible ink are two measurements, not one.** `sizeIn(min =
      44.dp)` bounds a box from below and nothing bounds it from above: a child's own intrinsic
      size inflates the box past the mockup's drawn size and past the floor itself, and reading
      the constant in code sees none of it. An accessibility minimum applies to the tap target
      only, while the mockup draws the visible shape, so a control that looks oversized is
      measured twice — hit box and ink — before either number is called wrong.

   **A first impression that a control "looks broken" is a hypothesis, not a finding — zoom and
   contrast-check the capture before it becomes a code change.** A small, low-contrast but
   pixel-correct element reads as wrong at a glance, and "hard to see" is a different question
   from "drawn wrong" — one for the operator, not for a fix.
   Symmetric with the measurement rule above: the eye that passes a control owes a number, and so
   does the eye that fails one.

   **"Looks compact", "looks tight", "matches the proportions" is the same hypothesis said of a
   gap — a spacing claim is backed by the script's `gap` property on every side it speaks for,
   never by a glance.** The eye cannot compare two absolute gaps in a scaled screenshot — a
   4x asymmetry between the space above a control and the space below it reads as "about the
   same" at 1x — so a claim about the space between two elements (a control and the dividers
   around it, a heading and its rule, a row and its neighbours) names the measured background
   pixels on each side, from one capture, and the mockup's on the same sides, before it is called
   symmetric, tight or matching. Gaps compound where sizes do not: an outer container's content
   padding stacks on a header's own inset on one side only, and no per-element size measurement
   sees it. An operator's re-raised or repeated spacing complaint is a measurement order, not a
   second look — the second look is what already failed.

   **"Filled to the border", "flush with the edge", "reaches the corner" is a third measurement,
   and neither of the two above sees it — a claim that one shape's paint ends where another
   shape's boundary is compares two edge coordinates, never one shape's size (hit box or ink)
   and never the empty space between two shapes (`gap`).** A fill inset from its own container's
   border has no gap the `gap` property can find — the inset is the container's own interior
   colour — and a correct box size says nothing about where the box sits. Crop the *container*
   as the region, centred on the filled row, and read `--props runs`: the run right after the
   border's run is the fill when flush, and the container-coloured run between them when not,
   its `length` the inset in px — the mockup's frame gives the same list to compare against.
   Not `content.padding`: it boxes every non-fill pixel in the container, the other rows' text
   included, and a fill covering half the box flips which colour counts as `fill`. The same
   reading covers a highlight against the row it highlights and an icon against the circle
   drawn behind it. In Compose the usual cause is modifier order alone — a `padding()` placed
   before the `background()` or `clip()` it was meant to inset the content of, or on the parent
   before its children's own backgrounds, shrinks the painted area, not the content — a
   checkable line of code before any screenshot.

## The pair to judge

- Mockup: /case/mockup.png
- Capture: /case/capture.png

Read both images and judge whether the capture matches the mockup. `measure-visual-properties.sh` is on PATH. The mockup is already cut to its frame, the same size as the capture, so measure the two with `--scale 1`; nothing is launched, captured, composed or written to a report file.

## Verdict

List each departure on its own line as `- defect: <what differs, and where>`, then end your reply with exactly one line: `verdict: match` or `verdict: mismatch`.
