package generator

import (
	"strings"
	"testing"
)

func TestRI_WithinInclusiveBounds(t *testing.T) {
	p := mustParse(t, `{"params":{"x":["ri",[-20,120]]}}`)
	src := newTestSource(1)
	for i := 0; i < 1000; i++ {
		v, err := p.FloatValue("x", src)
		if err != nil {
			t.Fatalf("FloatValue: %v", err)
		}
		if v < -20 || v > 120 || v != float64(int(v)) {
			t.Fatalf("x = %v, want integer in [-20,120]", v)
		}
	}
}

func TestRI_SingleValue(t *testing.T) {
	p := mustParse(t, `{"params":{"x":["ri",[5,5]]}}`)
	v, err := p.FloatValue("x", newTestSource(1))
	if err != nil || v != 5 {
		t.Fatalf("x = %v, %v; want 5", v, err)
	}
}

func TestRI_MinGreaterMaxFails(t *testing.T) {
	p := mustParse(t, `{"params":{"x":["ri",[10,5]]}}`)
	if _, err := p.Value("x", newTestSource(1)); err == nil {
		t.Fatal("ri [10,5]: want error, got nil")
	}
}

func TestRC_PicksOnlyListed(t *testing.T) {
	p := mustParse(t, `{"params":{"t":["rc",[[0,180]]]}}`)
	src := newTestSource(1)
	seen := map[float64]bool{}
	for i := 0; i < 200; i++ {
		v, err := p.FloatValue("t", src)
		if err != nil {
			t.Fatalf("FloatValue: %v", err)
		}
		if v != 0 && v != 180 {
			t.Fatalf("t = %v, want 0 or 180", v)
		}
		seen[v] = true
	}
	if !seen[0] || !seen[180] {
		t.Errorf("nicht beide Werte gezogen: %v", seen)
	}
}

func TestRC_Strings(t *testing.T) {
	p := mustParse(t, `{"params":{"f":["rc",[["rect","circle"]]]}}`)
	src := newTestSource(1)
	for i := 0; i < 50; i++ {
		s, err := p.StringValue("f", src)
		if err != nil {
			t.Fatalf("StringValue: %v", err)
		}
		if s != "rect" && s != "circle" {
			t.Fatalf("f = %q", s)
		}
	}
}

func TestRC_EmptyListFails(t *testing.T) {
	p := mustParse(t, `{"params":{"f":["rc",[[]]]}}`)
	if _, err := p.Value("f", newTestSource(1)); err == nil {
		t.Fatal("rc [[]]: want error, got nil")
	}
}

func TestUnknownRuleFails(t *testing.T) {
	p := mustParse(t, `{"params":{"f":["xx",[1]]}}`)
	_, err := p.Value("f", newTestSource(1))
	if err == nil || !strings.Contains(err.Error(), "xx") {
		t.Fatalf("err = %v, want error naming the rule", err)
	}
}

func TestRI_HugeBoundsFail(t *testing.T) {
	for _, bounds := range []string{"[-9e18,9e18]", "[0,1e300]", "[-3000000000,0]"} {
		t.Run(bounds, func(t *testing.T) {
			p := mustParse(t, `{"params":{"x":["ri",`+bounds+`]}}`)
			if _, err := p.Value("x", newTestSource(1)); err == nil {
				t.Fatalf("ri %s: want error, got nil", bounds)
			}
		})
	}
}

func TestSameSeedSameValues(t *testing.T) {
	p := mustParse(t, `{"params":{"x":["ri",[0,1000]]}}`)
	a, b := newTestSource(7), newTestSource(7)
	for i := 0; i < 20; i++ {
		va, _ := p.FloatValue("x", a)
		vb, _ := p.FloatValue("x", b)
		if va != vb {
			t.Fatalf("Ziehung %d: %v != %v", i, va, vb)
		}
	}
}
