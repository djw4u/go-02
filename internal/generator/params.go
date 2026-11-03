// Package generator erzeugt aus JSON-Parametern SVG-Bilder mit geometrischen
// Figuren. Es kennt weder HTTP noch die Datenbank.
package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// Source ist die injizierte Zufallsquelle; *rand.Rand aus math/rand/v2
// erfüllt sie.
type Source interface {
	IntN(n int) int
	Float64() float64
}

// Params sind die Parameter eines Bildes (der Inhalt von "params").
type Params struct {
	fields map[string]any
}

// Parse liest eine Parameterdatei der Form {"params":{...}}.
func Parse(raw []byte) (Params, error) {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return Params{}, fmt.Errorf("parse params: %w", err)
	}
	inner, ok := top["params"]
	if !ok {
		return Params{}, errors.New(`parse params: missing key "params"`)
	}
	fields, ok := inner.(map[string]any)
	if !ok {
		return Params{}, errors.New(`parse params: "params" is not an object`)
	}
	return Params{fields: fields}, nil
}

// Has meldet, ob das Feld vorhanden ist.
func (p Params) Has(name string) bool {
	_, ok := p.fields[name]
	return ok
}

// Value liefert den Wert des Feldes: ein fester Wert wird unverändert
// zurückgegeben, eine Regel wird mit src ausgewertet.
func (p Params) Value(name string, src Source) (any, error) {
	v, ok := p.fields[name]
	if !ok {
		return nil, fmt.Errorf("missing field %q", name)
	}
	if rule, args, ok := asRule(v); ok {
		out, err := evalRule(rule, args, src)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", name, err)
		}
		return out, nil
	}
	return v, nil
}

// FloatValue wie Value, das Ergebnis muss eine Zahl sein.
func (p Params) FloatValue(name string, src Source) (float64, error) {
	v, err := p.Value(name, src)
	if err != nil {
		return 0, err
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	}
	return 0, fmt.Errorf("field %q: want number, got %T", name, v)
}

// StringValue wie Value, das Ergebnis muss ein Text sein.
func (p Params) StringValue(name string, src Source) (string, error) {
	v, err := p.Value(name, src)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %q: want string, got %T", name, v)
	}
	return s, nil
}

// FixedFloat liefert eine feste Zahl; Regeln sind hier nicht erlaubt.
func (p Params) FixedFloat(name string) (float64, error) {
	v, ok := p.fields[name]
	if !ok {
		return 0, fmt.Errorf("missing field %q", name)
	}
	if _, _, isRule := asRule(v); isRule {
		return 0, fmt.Errorf("field %q: rule not allowed, want fixed number", name)
	}
	n, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("field %q: want number, got %T", name, v)
	}
	return n, nil
}

// FixedInt liefert eine feste ganze Zahl; Regeln sind hier nicht erlaubt.
func (p Params) FixedInt(name string) (int, error) {
	n, err := p.FixedFloat(name)
	if err != nil {
		return 0, err
	}
	if n != math.Trunc(n) {
		return 0, fmt.Errorf("field %q: want integer, got %v", name, n)
	}
	return int(n), nil
}
