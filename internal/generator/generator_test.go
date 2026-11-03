package generator

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const randomParams = `{"params":{"nr_of_figures":32,
 "figures":["rc",[["rect","polygon","circle","line"]]],
 "x":["ri",[-20,120]],"y":["ri",[-20,120]],
 "fill":["rc",[["#4353D3","#32D761","#D5C734","#D532CE","#D83144"]]],
 "transform":["rc",[[0,180]]],"structure":"RANDOM",
 "opacity":["rc",[[0.6,0.4]]],"height":["rc",[[40,20]]],"width":["rc",[[50,10]]],
 "palette":["#4353D3","#32D761","#D5C734","#D532CE","#D83144"],
 "stroke":["rc",[["#4353D3","#32D761","#D5C734","#D532CE","#D83144"]]],
 "stroke-linecap":["rc",[["round"]]],"stroke-width":["rc",[[6,4]]],
 "stroke-opacity":["rc",[[0.8,0.6]]]}}`

var (
	figureElement = regexp.MustCompile(`<(rect|circle|polygon|line)\b`)
	centerPoint   = regexp.MustCompile(`cx="([-0-9.]+)" cy="([-0-9.]+)"`)
)

func generate(t *testing.T, raw string, seed uint64) string {
	t.Helper()
	svg, err := Generate(mustParse(t, raw), newTestSource(seed))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return svg
}

func TestGenerate_Random_CountAndKinds(t *testing.T) {
	svg := generate(t, randomParams, 1)

	if !strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">`) {
		t.Errorf("unerwarteter Anfang: %.80s", svg)
	}
	if !strings.HasSuffix(strings.TrimSpace(svg), `</svg>`) {
		t.Errorf("unerwartetes Ende: %s", svg[max(0, len(svg)-40):])
	}
	if got := len(figureElement.FindAllString(svg, -1)); got != 32 {
		t.Errorf("Figuren = %d, want 32", got)
	}
}

func TestGenerate_SameSeedSameSVG(t *testing.T) {
	if a, b := generate(t, randomParams, 1), generate(t, randomParams, 1); a != b {
		t.Error("gleicher Seed lieferte unterschiedliche SVGs")
	}
}

func TestGenerate_DifferentSeedDiffers(t *testing.T) {
	if a, b := generate(t, randomParams, 1), generate(t, randomParams, 2); a == b {
		t.Error("unterschiedliche Seeds lieferten dasselbe SVG")
	}
}

func TestGenerate_Grid(t *testing.T) {
	svg := generate(t, `{"params":{"structure":"GRID","rows":2,"columns":2,"figures":"circle","width":10}}`, 1)

	want := []string{`cx="25" cy="25"`, `cx="75" cy="25"`, `cx="25" cy="75"`, `cx="75" cy="75"`}
	last := -1
	for _, w := range want {
		i := strings.Index(svg, w)
		if i <= last {
			t.Fatalf("%s fehlt oder steht in falscher Reihenfolge in:\n%s", w, svg)
		}
		last = i
	}
	if got := len(figureElement.FindAllString(svg, -1)); got != 4 {
		t.Errorf("Figuren = %d, want 4", got)
	}
}

func TestGenerate_PDS_MinDistance(t *testing.T) {
	svg := generate(t, `{"params":{"structure":"PDS","nr_of_figures":20,"min_distance":10,"figures":"circle","width":4}}`, 1)

	matches := centerPoint.FindAllStringSubmatch(svg, -1)
	if len(matches) == 0 || len(matches) > 20 {
		t.Fatalf("Figuren = %d, want 1..20", len(matches))
	}
	pts := make([][2]float64, len(matches))
	for i, m := range matches {
		pts[i][0], _ = strconv.ParseFloat(m[1], 64)
		pts[i][1], _ = strconv.ParseFloat(m[2], 64)
	}
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			// Ausgabe ist auf 3 Nachkommastellen gerundet.
			if d := math.Hypot(pts[i][0]-pts[j][0], pts[i][1]-pts[j][1]); d < 10-0.01 {
				t.Errorf("Abstand %d-%d = %v, want >= 10", i, j, d)
			}
		}
	}
}

func TestGenerate_PDS_ImpossibleDensityTerminates(t *testing.T) {
	svg := generate(t, `{"params":{"structure":"PDS","nr_of_figures":10,"min_distance":200,"figures":"circle","width":4}}`, 1)
	if got := len(figureElement.FindAllString(svg, -1)); got != 1 {
		t.Errorf("Figuren = %d, want 1", got)
	}
}

func TestGenerate_Errors(t *testing.T) {
	tests := []struct {
		name    string
		fields  string
		wantErr string
	}{
		{"structure fehlt", `{"figures":"circle","width":10,"nr_of_figures":3}`, "structure"},
		{"unbekannte structure", `{"structure":"SPIRAL","figures":"circle","width":10}`, "SPIRAL"},
		{"figures fehlt", `{"structure":"RANDOM","nr_of_figures":1,"x":1,"y":1,"width":10}`, "figures"},
		{"ungültige Figurenart", `{"structure":"RANDOM","nr_of_figures":1,"x":1,"y":1,"figures":"hexagon","width":10}`, "hexagon"},
		{"RANDOM nr_of_figures 0", `{"structure":"RANDOM","nr_of_figures":0,"x":1,"y":1,"figures":"circle","width":10}`, "nr_of_figures"},
		{"RANDOM zu viele Figuren", `{"structure":"RANDOM","nr_of_figures":10001,"x":1,"y":1,"figures":"circle","width":10}`, "nr_of_figures"},
		{"GRID rows fehlt", `{"structure":"GRID","columns":2,"figures":"circle","width":10}`, "rows"},
		{"GRID rows 0", `{"structure":"GRID","rows":0,"columns":2,"figures":"circle","width":10}`, "rows"},
		{"GRID zu viele Zellen", `{"structure":"GRID","rows":101,"columns":100,"figures":"circle","width":10}`, "rows"},
		{"PDS min_distance 0", `{"structure":"PDS","nr_of_figures":3,"min_distance":0,"figures":"circle","width":10}`, "min_distance"},
		{"PDS min_distance fehlt", `{"structure":"PDS","nr_of_figures":3,"figures":"circle","width":10}`, "min_distance"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := mustParse(t, `{"params":`+tt.fields+`}`)
			_, err := Generate(p, newTestSource(1))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
