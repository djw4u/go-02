package generator

import (
	"fmt"
	"math"
)

// asRule erkennt eine Regel: ein Array der Länge 2 mit einem Text an Stelle 0
// und einem Array an Stelle 1.
// maxRuleBound begrenzt die Grenzen der Regel "ri", damit die Spanne sicher in
// einen int passt.
const maxRuleBound = 1 << 31

func asRule(v any) (name string, args []any, ok bool) {
	arr, isArr := v.([]any)
	if !isArr || len(arr) != 2 {
		return "", nil, false
	}
	name, isStr := arr[0].(string)
	args, isArgs := arr[1].([]any)
	if !isStr || !isArgs {
		return "", nil, false
	}
	return name, args, true
}

func evalRule(name string, args []any, src Source) (any, error) {
	switch name {
	case "ri":
		return ruleRandomInt(args, src)
	case "rc":
		return ruleRandomChoice(args, src)
	default:
		return nil, fmt.Errorf("unknown rule %q", name)
	}
}

// ruleRandomInt: ["ri",[min,max]] liefert eine Ganzzahl, Grenzen inklusive.
func ruleRandomInt(args []any, src Source) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("rule ri: want [min,max], got %d arguments", len(args))
	}
	lo, okLo := args[0].(float64)
	hi, okHi := args[1].(float64)
	if !okLo || !okHi || lo != math.Trunc(lo) || hi != math.Trunc(hi) {
		return nil, fmt.Errorf("rule ri: min and max must be integers, got %v", args)
	}
	if math.Abs(lo) > maxRuleBound || math.Abs(hi) > maxRuleBound {
		return nil, fmt.Errorf("rule ri: bounds must be within ±%d, got %v", int64(maxRuleBound), args)
	}
	if lo > hi {
		return nil, fmt.Errorf("rule ri: min %v is greater than max %v", lo, hi)
	}
	return int(lo) + src.IntN(int(hi)-int(lo)+1), nil
}

// ruleRandomChoice: ["rc",[[a,b,...]]] liefert ein Element der Liste.
func ruleRandomChoice(args []any, src Source) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("rule rc: want [[choices]], got %d arguments", len(args))
	}
	choices, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("rule rc: first argument must be a list, got %T", args[0])
	}
	if len(choices) == 0 {
		return nil, fmt.Errorf("rule rc: list of choices is empty")
	}
	return choices[src.IntN(len(choices))], nil
}
