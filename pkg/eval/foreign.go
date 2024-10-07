package eval

import (
	"encoding/json"
	"fmt"
	"math"
)

func ValueFromNative(x any) NixValue {
	switch t := x.(type) {
	case float64:
		if math.Mod(t, 1.0) == 0 {
			return NixInt(int64(t))
		} else {
			return NixFloat(t)
		}
	case bool:
		return NixBool(t)
	case string:
		return &NixString{Content: t}
	case []any:
		result := make(NixList, 0, len(t))
		for _, val := range t {
			result = append(result, &Expression{Value: ValueFromNative(val)})
		}
		return result
	case map[string]any:
		result := make(NixSet, len(t))
		for key, val := range t {
			result[Intern(key)] = &Expression{Value: ValueFromNative(val)}
		}
		return result
	default:
		panic(fmt.Sprintf("unsupported native type %T!", t))
	}
}

func ValueToNative(x NixValue) any {
	switch t := x.(type) {
	case NixInt:
		return int64(t)
	case NixFloat:
		return float64(t)
	case NixBool:
		return bool(t)
	case *NixString:
		return t.Content
	case NixList:
		result := make([]any, 0, len(t))
		for _, expr := range t {
			result = append(result, ValueToNative(expr.Eval()))
		}
		return result
	case NixSet:
		result := make(map[string]any, len(t))
		for sym, expr := range t {
			result[sym.String()] = ValueToNative(expr.Eval())
		}
		return result
	default:
		panic(fmt.Sprintf("unsupported nix type %T!", t))
	}
}

func ValueFromJSON(x *NixString) NixValue {
	var c any
	err := json.Unmarshal([]byte(x.Content), &c)
	if err != nil {
		panic(err)
	}
	return ValueFromNative(c)
}

func ValueToJSON(x NixValue) *NixString {
	b, err := json.Marshal(ValueToNative(x))
	if err != nil {
		panic(err)
	}
	return &NixString{Content: string(b)}
}
