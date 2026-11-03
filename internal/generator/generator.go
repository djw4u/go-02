package generator

import (
	"fmt"
	"strings"
)

// Generate erzeugt aus den Parametern ein SVG mit der Zeichenfläche
// "0 0 100 100". Ungültige Parameter führen zu einem Fehler, nie zu einem
// stillen Ergebnis.
func Generate(p Params, src Source) (string, error) {
	structure, err := p.StringValue("structure", src)
	if err != nil {
		return "", err
	}
	place, ok := layoutFor(structure)
	if !ok {
		return "", fmt.Errorf("unknown structure %q", structure)
	}
	points, err := place(p, src)
	if err != nil {
		return "", fmt.Errorf("structure %s: %w", structure, err)
	}

	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` + "\n")
	for _, pt := range points {
		kind, err := p.StringValue("figures", src)
		if err != nil {
			return "", err
		}
		el, err := drawFigure(kind, pt.X, pt.Y, p, src)
		if err != nil {
			return "", err
		}
		b.WriteString(el + "\n")
	}
	b.WriteString("</svg>\n")
	return b.String(), nil
}
