# Brand assets

Two links of a chain, hooked through each other: iron for the first, ember
for the second, each one left open. Open links rather than closed rings
because a pair of closed interlocking rings is the universal hyperlink
glyph and says nothing about this library — while an open link is the
honest picture of a lazy sequence, built but not yet closed.

**These files are generated. Do not edit them by hand.** They are computed
from a handful of ratios in `internal/gen/assets`; change a constant there
and run `go generate ./...`. CI fails if the committed files and the
generator disagree.

| File | Use |
|---|---|
| `icon.svg` | on light grounds — favicon, README, docs header |
| `icon-dark.svg` | the same mark, lifted for dark grounds |

## Palette

| | Light | Dark | Role |
|---|---|---|---|
| Iron | `#33475B` | `#5C7A96` | the first link, structure |
| Ember | `#D9642B` | `#E07B45` | the second link, accents |

`#D9642B` is a graphic fill, **not** the text accent: it measures below
4.5:1 on paper, so links in prose use a darkened `#B4501F`. On dark
grounds the lifted ember serves as both.

## Why the proportions are what they are

They are load-bearing, and tuning them by eye does not work — a mark that
reads as two linked rings at a glance collapses into one blob if the
hole-to-band ratio drifts much below 1.5, or if the axis gets much steeper
than about 35°. The generator refuses to emit a mark whose opening
overflows its straight run, or whose links sit too far apart to interlock,
because both produce something that looks like a rendering fault rather
than a chain. If the mark needs changing, change one ratio and look at the
result at 16 and 32 pixels before keeping it.
