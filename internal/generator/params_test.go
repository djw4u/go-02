package generator

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func newTestSource(seed uint64) Source {
	return rand.New(rand.NewPCG(seed, seed+1))
}

func mustParse(t *testing.T, raw string) Params {
	t.Helper()
	p, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse(%s): %v", raw, err)
	}
	return p
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"ungültiges JSON", `{`},
		{"fehlender Schlüssel params", `{}`},
		{"params kein Objekt", `{"params":5}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse([]byte(tt.raw)); err == nil {
				t.Fatalf("Parse(%s): want error, got nil", tt.raw)
			}
		})
	}
}

func TestValue_FixedPassesThrough(t *testing.T) {
	p := mustParse(t, `{"params":{"structure":"RANDOM","nr_of_figures":32}}`)

	got, err := p.Value("structure", newTestSource(1))
	if err != nil || got != "RANDOM" {
		t.Errorf("structure = %v, %v; want RANDOM", got, err)
	}
	n, err := p.FixedInt("nr_of_figures")
	if err != nil || n != 32 {
		t.Errorf("FixedInt(nr_of_figures) = %d, %v; want 32", n, err)
	}
	if !p.Has("structure") || p.Has("fehlt") {
		t.Error("Has liefert falsche Ergebnisse")
	}
}

func TestMissingFieldFails(t *testing.T) {
	p := mustParse(t, `{"params":{}}`)
	_, err := p.Value("width", newTestSource(1))
	if err == nil || !strings.Contains(err.Error(), "width") {
		t.Fatalf("err = %v, want error naming the field", err)
	}
}

func TestPaletteIsNotARule(t *testing.T) {
	p := mustParse(t, `{"params":{"palette":["#4353D3","#32D761","#D5C734","#D532CE","#D83144"],"duo":["#000","#fff"]}}`)
	for _, name := range []string{"palette", "duo"} {
		got, err := p.Value(name, newTestSource(1))
		if err != nil {
			t.Fatalf("Value(%s): %v", name, err)
		}
		if _, ok := got.([]any); !ok {
			t.Errorf("Value(%s) = %T, want fester Wert []any", name, got)
		}
	}
}

func TestFixedValues_RejectWrongTypes(t *testing.T) {
	p := mustParse(t, `{"params":{"a":1.5,"s":"x","r":["ri",[1,2]]}}`)
	if _, err := p.FixedInt("a"); err == nil {
		t.Error("FixedInt(1.5): want error")
	}
	if _, err := p.FixedInt("s"); err == nil {
		t.Error("FixedInt(string): want error")
	}
	if _, err := p.FixedFloat("r"); err == nil {
		t.Error("FixedFloat(rule): want error")
	}
	if f, err := p.FixedFloat("a"); err != nil || f != 1.5 {
		t.Errorf("FixedFloat(a) = %v, %v; want 1.5", f, err)
	}
}
