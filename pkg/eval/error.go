package eval

import (
	"fmt"
)

func PrintType(val NixValue) string {
	switch val.(type) {
		case NixInt:
			return "int"
		case NixFloat:
			return "float"
		case NixBool:
			return "bool"
		case *NixNull:
			return "null"
		case NixList:
			return "list"
		case NixSet:
			return "set"
		case *NixPath:
			return "path"
		case *NixString:
			return "string"
		default: // if T below is `NixLambda`, type will be nil
			return "lambda"
	}
}

func AssertType[T NixValue](val NixValue) T {
	if tVal, ok := val.(T); ok {
		return tVal
	} else {
		var t T
		panic(fmt.Sprintf("Expected a %s but found %s: %s", PrintType(t), PrintType(val), val.Print(false)))
	}
}
