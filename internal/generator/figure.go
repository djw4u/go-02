package generator

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
)

// styleAttr ist ein optionales SVG-Attribut, das nur ausgegeben wird, wenn das
// gleichnamige Parameterfeld vorhanden ist.
type styleAttr struct {
	name    string
	numeric bool
}

// styleAttrs gibt die Reihenfolge der optionalen Attribute vor.
var styleAttrs = []styleAttr{
	{"fill", false},
	{"opacity", true},
	{"stroke", false},
	{"stroke-width", true},
	{"stroke-opacity", true},
	{"stroke-linecap", false},
}

const (
	defaultSides = 5
	// maxSides begrenzt die Eckenzahl eines Polygons, damit das SVG klein bleibt.
	maxSides = 1000
)

// drawFigure liefert das SVG-Element einer Figur mit Mittelpunkt (x, y).
func drawFigure(kind string, x, y float64, p Params, src Source) (string, error) {
	switch kind {
	case "rect", "square", "circle", "line", "polygon", "triangle":
	default:
		return "", fmt.Errorf("unknown figure kind %q", kind)
	}

	width, err := p.FloatValue("width", src)
	if err != nil {
		return "", fmt.Errorf("figure %q: %w", kind, err)
	}

	var shape string
	switch kind {
	case "rect":
		height, err := p.FloatValue("height", src)
		if err != nil {
			return "", fmt.Errorf("figure %q: %w", kind, err)
		}
		shape = rectElement(x, y, width, height)
	case "square":
		shape = rectElement(x, y, width, width)
	case "circle":
		shape = fmt.Sprintf(`<circle cx="%s" cy="%s" r="%s"`, num(x), num(y), num(width/2))
	case "line":
		shape = fmt.Sprintf(`<line x1="%s" y1="%s" x2="%s" y2="%s"`,
			num(x-width/2), num(y), num(x+width/2), num(y))
	case "triangle":
		shape = polygonElement(x, y, width/2, 3)
	case "polygon":
		sides, err := polygonSides(p, src)
		if err != nil {
			return "", fmt.Errorf("figure %q: %w", kind, err)
		}
		shape = polygonElement(x, y, width/2, sides)
	}

	attrs, err := styleAttributes(kind, x, y, p, src)
	if err != nil {
		return "", fmt.Errorf("figure %q: %w", kind, err)
	}
	return shape + attrs + "/>", nil
}

func rectElement(x, y, w, h float64) string {
	return fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s"`,
		num(x-w/2), num(y-h/2), num(w), num(h))
}

// polygonElement zeichnet ein regelmäßiges n-Eck mit Umkreisradius r, der
// erste Punkt liegt oben.
func polygonElement(x, y, r float64, n int) string {
	points := make([]string, n)
	for k := range points {
		angle := -math.Pi/2 + 2*math.Pi*float64(k)/float64(n)
		points[k] = num(x+r*math.Cos(angle)) + "," + num(y+r*math.Sin(angle))
	}
	return `<polygon points="` + strings.Join(points, " ") + `"`
}

func polygonSides(p Params, src Source) (int, error) {
	if !p.Has("sides") {
		return defaultSides, nil
	}
	v, err := p.FloatValue("sides", src)
	if err != nil {
		return 0, err
	}
	if v != math.Trunc(v) || v < 3 || v > maxSides {
		return 0, fmt.Errorf("field %q: want integer 3..%d, got %v", "sides", maxSides, v)
	}
	return int(v), nil
}

func styleAttributes(kind string, x, y float64, p Params, src Source) (string, error) {
	var b strings.Builder
	for _, a := range styleAttrs {
		if !p.Has(a.name) || (kind == "line" && a.name == "fill") {
			continue
		}
		if a.numeric {
			v, err := p.FloatValue(a.name, src)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, ` %s="%s"`, a.name, num(v))
			continue
		}
		s, err := p.StringValue(a.name, src)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, ` %s="%s"`, a.name, html.EscapeString(s))
	}
	if p.Has("transform") {
		angle, err := p.FloatValue("transform", src)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, ` transform="rotate(%s %s %s)"`, num(angle), num(x), num(y))
	}
	return b.String(), nil
}

// num formatiert eine Zahl mit höchstens drei Nachkommastellen ohne
// nachgestellte Nullen.
func num(v float64) string {
	r := math.Round(v*1000) / 1000
	if r == 0 {
		return "0"
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}
