package generator

import (
	"fmt"
	"math"
)

const (
	// canvasSize ist die Kantenlänge der quadratischen Zeichenfläche.
	canvasSize = 100.0
	// maxFigures begrenzt die Anzahl der Figuren pro Bild.
	maxFigures = 10000
	// pdsAttemptsPerFigure begrenzt die Versuche der PDS-Anordnung je Figur.
	pdsAttemptsPerFigure = 100
)

type point struct{ X, Y float64 }

// layout bestimmt die Mittelpunkte aller Figuren.
type layout func(p Params, src Source) ([]point, error)

// layoutFor liefert die Anordnung zum Namen aus dem Feld "structure". Eine
// neue Anordnung ist ein weiterer case.
func layoutFor(structure string) (layout, bool) {
	switch structure {
	case "RANDOM":
		return randomLayout, true
	case "GRID":
		return gridLayout, true
	case "PDS":
		return pdsLayout, true
	}
	return nil, false
}

// figureCount liest nr_of_figures und prüft den erlaubten Bereich.
func figureCount(p Params) (int, error) {
	n, err := p.FixedInt("nr_of_figures")
	if err != nil {
		return 0, err
	}
	if n < 1 || n > maxFigures {
		return 0, fmt.Errorf("field %q: want 1..%d, got %d", "nr_of_figures", maxFigures, n)
	}
	return n, nil
}

// randomLayout: x und y jeder Figur kommen aus den Regeln der Felder x und y.
func randomLayout(p Params, src Source) ([]point, error) {
	n, err := figureCount(p)
	if err != nil {
		return nil, err
	}
	points := make([]point, n)
	for i := range points {
		x, err := p.FloatValue("x", src)
		if err != nil {
			return nil, err
		}
		y, err := p.FloatValue("y", src)
		if err != nil {
			return nil, err
		}
		points[i] = point{x, y}
	}
	return points, nil
}

// gridLayout: Zellmittelpunkte eines rows x columns Rasters, zeilenweise.
func gridLayout(p Params, _ Source) ([]point, error) {
	rows, err := p.FixedInt("rows")
	if err != nil {
		return nil, err
	}
	columns, err := p.FixedInt("columns")
	if err != nil {
		return nil, err
	}
	if rows < 1 || columns < 1 {
		return nil, fmt.Errorf("fields %q and %q: want >= 1, got %d and %d", "rows", "columns", rows, columns)
	}
	if rows > maxFigures/columns {
		return nil, fmt.Errorf("fields %q and %q: more than %d cells", "rows", "columns", maxFigures)
	}
	points := make([]point, 0, rows*columns)
	for r := 0; r < rows; r++ {
		for c := 0; c < columns; c++ {
			points = append(points, point{
				X: (float64(c) + 0.5) * canvasSize / float64(columns),
				Y: (float64(r) + 0.5) * canvasSize / float64(rows),
			})
		}
	}
	return points, nil
}

// pdsLayout: zufällige Punkte in der Zeichenfläche mit Mindestabstand
// min_distance (Dart-Throwing). Passen nicht alle Punkte hinein, wird die
// kleinere Zahl ohne Fehler geliefert.
func pdsLayout(p Params, src Source) ([]point, error) {
	n, err := figureCount(p)
	if err != nil {
		return nil, err
	}
	minDist, err := p.FixedFloat("min_distance")
	if err != nil {
		return nil, err
	}
	if minDist <= 0 {
		return nil, fmt.Errorf("field %q: want > 0, got %v", "min_distance", minDist)
	}

	points := make([]point, 0, n)
	for attempt := 0; attempt < pdsAttemptsPerFigure*n && len(points) < n; attempt++ {
		cand := point{src.Float64() * canvasSize, src.Float64() * canvasSize}
		if farEnough(cand, points, minDist) {
			points = append(points, cand)
		}
	}
	return points, nil
}

func farEnough(cand point, points []point, minDist float64) bool {
	for _, q := range points {
		if math.Hypot(cand.X-q.X, cand.Y-q.Y) < minDist {
			return false
		}
	}
	return true
}
