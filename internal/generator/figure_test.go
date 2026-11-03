package generator

import (
	"strings"
	"testing"
)

func figureParams(t *testing.T, fields string) Params {
	t.Helper()
	return mustParse(t, `{"params":`+fields+`}`)
}

func TestNum(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{50, "50"},
		{12.5, "12.5"},
		{58.660254, "58.66"},
		{-0.0001, "0"},
		{0.1 + 0.2, "0.3"},
		{-20, "-20"},
	}
	for _, tt := range tests {
		if got := num(tt.in); got != tt.want {
			t.Errorf("num(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDrawFigure_Shapes(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		fields string
		want   string
	}{
		{
			"rect",
			"rect",
			`{"width":40,"height":20,"fill":"#4353D3","transform":180}`,
			`<rect x="30" y="40" width="40" height="20" fill="#4353D3" transform="rotate(180 50 50)"/>`,
		},
		{
			"square",
			"square",
			`{"width":10}`,
			`<rect x="45" y="45" width="10" height="10"/>`,
		},
		{
			"circle",
			"circle",
			`{"width":10}`,
			`<circle cx="50" cy="50" r="5"/>`,
		},
		{
			"line ohne fill",
			"line",
			`{"width":20,"fill":"#f00","stroke":"#000","stroke-width":2}`,
			`<line x1="40" y1="50" x2="60" y2="50" stroke="#000" stroke-width="2"/>`,
		},
		{
			"triangle",
			"triangle",
			`{"width":20}`,
			`<polygon points="50,40 58.66,55 41.34,55"/>`,
		},
		{
			"polygon mit sides",
			"polygon",
			`{"width":20,"sides":4}`,
			`<polygon points="50,40 60,50 50,60 40,50"/>`,
		},
		{
			"attribut-reihenfolge",
			"circle",
			`{"transform":45,"stroke-linecap":"round","stroke-opacity":0.8,"stroke-width":2,"stroke":"#222","opacity":0.5,"fill":"#111","width":10}`,
			`<circle cx="50" cy="50" r="5" fill="#111" opacity="0.5" stroke="#222" stroke-width="2" stroke-opacity="0.8" stroke-linecap="round" transform="rotate(45 50 50)"/>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := figureParams(t, tt.fields)
			got, err := drawFigure(tt.kind, 50, 50, p, newTestSource(1))
			if err != nil {
				t.Fatalf("drawFigure: %v", err)
			}
			if got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestDrawFigure_AppliesRules(t *testing.T) {
	p := figureParams(t, `{"width":["ri",[10,10]],"fill":["rc",[["#a","#b"]]]}`)
	got, err := drawFigure("circle", 50, 50, p, newTestSource(1))
	if err != nil {
		t.Fatalf("drawFigure: %v", err)
	}
	if !strings.Contains(got, `r="5"`) {
		t.Errorf("Radius aus Regel fehlt: %s", got)
	}
	if !strings.Contains(got, `fill="#a"`) && !strings.Contains(got, `fill="#b"`) {
		t.Errorf("fill aus Regel fehlt: %s", got)
	}
}

func TestDrawFigure_EscapesAttributeValues(t *testing.T) {
	p := figureParams(t, `{"width":10,"fill":"a\"b<c"}`)
	got, err := drawFigure("circle", 50, 50, p, newTestSource(1))
	if err != nil {
		t.Fatalf("drawFigure: %v", err)
	}
	want := `<circle cx="50" cy="50" r="5" fill="a&#34;b&lt;c"/>`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestDrawFigure_TooManySidesFails(t *testing.T) {
	p := figureParams(t, `{"width":10,"sides":1001}`)
	_, err := drawFigure("polygon", 50, 50, p, newTestSource(1))
	if err == nil || !strings.Contains(err.Error(), "sides") {
		t.Fatalf("err = %v, want error naming sides", err)
	}
	p = figureParams(t, `{"width":10,"sides":1000}`)
	if _, err := drawFigure("polygon", 50, 50, p, newTestSource(1)); err != nil {
		t.Fatalf("sides 1000: %v", err)
	}
}

func TestDrawFigure_Errors(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		fields  string
		wantErr string
	}{
		{"unbekannte Art", "hexagon", `{"width":10}`, "hexagon"},
		{"width fehlt", "circle", `{}`, "width"},
		{"rect ohne height", "rect", `{"width":10}`, "height"},
		{"sides zu klein", "polygon", `{"width":10,"sides":2}`, "sides"},
		{"sides keine Ganzzahl", "polygon", `{"width":10,"sides":3.5}`, "sides"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := figureParams(t, tt.fields)
			_, err := drawFigure(tt.kind, 50, 50, p, newTestSource(1))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
