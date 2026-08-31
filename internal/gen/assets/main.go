// Command assets generates the catena mark as SVG.
//
// The mark is two links of a chain, each one open, hooked through each
// other: iron for the first, ember for the second. Open links rather than
// closed rings for two reasons — a pair of closed interlocking rings is
// the universal hyperlink glyph and says nothing about this library, and
// an open link is the honest picture of a lazy sequence: the chain is
// built but not yet closed.
//
// Everything here is computed geometry. Do not edit the generated SVGs by
// hand — change a ratio below and re-run `go generate ./...`. Proportions
// are expressed against the link, so the mark is rebalanced by changing
// one number rather than by nudging shapes.
//
// The numbers were chosen by rendering the mark down to 16 and 32 pixels
// and comparing: thinner strokes vanish at favicon size, heavier ones
// close up the links' holes, and a gap wider than about a quarter of the
// straight run stops reading as a link at all.
package main

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
)

// Geometry, in a 64-unit square. Ratios, not pixels.
//
// These proportions are load-bearing, and tuning them by eye does not
// work — a mark that reads as two linked rings at a glance turns into one
// blob with a colour laid over it if any of them drifts far. Change one,
// then look at the result at 16 and 32 pixels before keeping it.
//
//	link length : height   2.17
//	hole : band            1.89   below ~1.5 the rings merge into a mass
//	overlap                26% of link length
//	axis                   -34.2 degrees, shallow enough to read as a chain
const (
	box      = 64.0  // viewBox side
	linkLen  = 56.4  // link length, long axis
	linkHigh = 26.0  // link height, short axis
	stroke   = 9.0   // band width; linkHigh/stroke is the hole-to-band ratio
	tiltDeg  = -34.2 // tilt of the chain's axis
	gapDist  = 41.6  // distance between link centres along that axis
	gapFrac  = 0.17  // opening, as a fraction of the straight run
	gapPos   = 0.30  // opening's offset along the run, toward the inner end
	fillFrac = 0.95  // share of the frame the mark's extent occupies
)

// palette is one theme's pair of link colours.
type palette struct {
	name        string
	iron, ember string
}

var palettes = []palette{
	// On paper. Ember is a graphic fill here, not the text accent — it
	// measures below AA for body text, which is why the docs use a
	// darkened sibling for links and this file does not.
	{"icon", "#33475B", "#D9642B"},
	// On ink. Iron lifts so it does not disappear into a dark ground;
	// ember lifts with it so the pair keeps its contrast.
	{"icon-dark", "#5C7A96", "#E07B45"},
}

// fatal is a seam so tests can exercise main's failure path without
// exiting the process.
var fatal = log.Fatal

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	for _, p := range palettes {
		svg, err := mark(p)
		if err != nil {
			fatal(err)
			return
		}
		for _, out := range targets(dir, p.name) {
			if err := os.WriteFile(out, svg, 0o644); err != nil {
				fatal(err)
				return
			}
		}
	}
	fmt.Printf("assets: %d marks written\n", len(palettes))
}

// targets lists every place a mark is consumed: Assets/ is the brand
// source of truth, and the site serves its favicon from public/.
func targets(dir, name string) []string {
	return []string{
		filepath.Join(dir, "Assets", name+".svg"),
		filepath.Join(dir, "website", "public", name+".svg"),
	}
}

// linkPath is one open link's centreline: a rounded rectangle interrupted
// by a gap on its lower run. Butt caps turn that gap into two flat faces
// cut square across the link, which is what makes it read as an open link
// rather than a broken one.
func linkPath(length, height, frac, pos float64) (string, error) {
	r := height / 2
	half := length/2 - r // half the straight run
	width := 2 * half * frac
	centre := half * pos
	g1, g2 := centre-width/2, centre+width/2
	if g1 <= -half || g2 >= half {
		// Silently overflowing the run strands a fragment of stroke that
		// reads as a rendering fault, so it fails loudly instead.
		return "", fmt.Errorf("gap [%.1f, %.1f] overflows the straight run [%.1f, %.1f]: reduce gapFrac or lengthen the link",
			g1, g2, -half, half)
	}
	return fmt.Sprintf("M %.2f %.2f L %.2f %.2f A %g %g 0 0 1 %.2f %.2f L %.2f %.2f A %g %g 0 0 1 %.2f %.2f L %.2f %.2f",
		g1, r, -half, r, r, r, -half, -r, half, -r, r, r, half, r, g2, r), nil
}

// crossingReach is how far along the perpendicular the two links' outlines
// meet. Each link's inner end-cap is a circle of radius r whose centres sit
// 2e apart on the chain's axis, so the circles — and therefore the links —
// cross at ±sqrt(r²-e²). The weave needs that exactly: it must cover one
// crossing and not the other, or the mark reads as one colour laid over
// the other rather than as a chain.
func crossingReach(length, height, dist float64) (float64, error) {
	r := height / 2
	e := (dist - length + height) / 2
	if e >= r {
		return 0, fmt.Errorf("links do not interlock: end-caps are %.1f apart but only %.1f across; reduce gapDist or lengthen the link", 2*e, 2*r)
	}
	return math.Sqrt(r*r - e*e), nil
}

func mark(p palette) ([]byte, error) {
	d, err := linkPath(linkLen, linkHigh, gapFrac, gapPos)
	if err != nil {
		return nil, err
	}

	rad := tiltDeg * math.Pi / 180
	ux, uy := math.Cos(rad), math.Sin(rad)
	c := box / 2

	// Fit the mark to the frame from its own measured extent, rather than
	// from a scale factor tuned by hand — so changing the proportions
	// above can neither clip the stroke nor leave dead margin.
	axis, perp := gapDist+linkLen+stroke, linkHigh+stroke
	t := math.Abs(rad)
	width := axis*math.Cos(t) + perp*math.Sin(t)
	height := axis*math.Sin(t) + perp*math.Cos(t)
	scale := box * fillFrac / math.Max(width, height)

	// The two link centres, placed symmetrically about the mark's centre
	// along the chain's axis. The second link is the first rotated half a
	// turn, so the pair has rotational symmetry and both openings face
	// outward.
	ax, ay := c-gapDist/2*ux, c-gapDist/2*uy
	bx, by := c+gapDist/2*ux, c+gapDist/2*uy

	// Where the two centrelines actually cross. Each link's inner end-cap
	// is a circle of radius r; the two caps' centres lie `2e` apart on the
	// chain's axis, so the circles — and therefore the links — meet at
	// ±sqrt(r²-e²) along the perpendicular. Knowing this exactly is what
	// makes a weave possible: the mark needs iron over ember at ONE
	// crossing and under at the other. Covering both (which a disc merely
	// centred on the mark does) paints one colour over the other and the
	// chain stops reading as linked.
	reach, err := crossingReach(linkLen, linkHigh, gapDist)
	if err != nil {
		return nil, err
	}
	vx, vy := -math.Sin(rad), math.Cos(rad) // perpendicular to the axis
	crossX, crossY := c+reach*vx, c+reach*vy

	link := func(cx, cy, rot float64, colour string) string {
		return fmt.Sprintf(`<path d="%s" transform="translate(%.2f %.2f) rotate(%g)" stroke="%s"/>`,
			d, cx, cy, rot, colour)
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %g %g" width="%g" height="%g" role="img" aria-label="catena">`,
		box, box, box, box)
	b.WriteString(`<title>catena</title>`)
	// The weave. Ember is drawn last and so covers iron everywhere; the
	// mask cuts one slot out of it, in the shape of the iron band itself,
	// at one crossing only. Iron shows through there and ember stays on
	// top at the other crossing, which is what threading looks like.
	//
	// The slot is the iron path re-stroked slightly wider — so its edges
	// run parallel to the iron band and the crossing reads as a clean
	// pass. A disc was tried first and bulges: it cuts the band on a curve
	// that belongs to neither link.
	fmt.Fprintf(&b, `<defs><clipPath id="near"><circle cx="%.2f" cy="%.2f" r="%.2f"/></clipPath>`,
		crossX, crossY, stroke*1.8)
	fmt.Fprintf(&b, `<mask id="weave" maskUnits="userSpaceOnUse" maskContentUnits="userSpaceOnUse" x="%g" y="%g" width="%g" height="%g">`,
		-box, -box, box*3, box*3)
	fmt.Fprintf(&b, `<rect x="%g" y="%g" width="%g" height="%g" fill="#fff"/>`, -box, -box, box*3, box*3)
	fmt.Fprintf(&b, `<g clip-path="url(#near)"><path d="%s" transform="translate(%.2f %.2f) rotate(%g)" fill="none" stroke="#000" stroke-width="%.2f" stroke-linecap="butt"/></g>`,
		d, ax, ay, tiltDeg, stroke+1.8)
	b.WriteString(`</mask></defs>`)
	fmt.Fprintf(&b, `<g transform="translate(%g %g) scale(%g) translate(%g %g)" fill="none" stroke-width="%g" stroke-linecap="butt" stroke-linejoin="round">`,
		c, c, scale, -c, -c, stroke)
	b.WriteString(link(ax, ay, tiltDeg, p.iron))
	// The mask rides a wrapper, not the path: an element's own transform
	// establishes the space its mask is resolved in, so masking the path
	// directly would place the slot inside the link's rotated local frame.
	fmt.Fprintf(&b, `<g mask="url(#weave)">%s</g>`, link(bx, by, tiltDeg+180, p.ember))
	b.WriteString(`</g></svg>` + "\n")
	return b.Bytes(), nil
}
