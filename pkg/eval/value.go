package eval

// TODO: InterpString

import (
	"fmt"
	p "github.com/orivej/go-nix/pkg/parser"
	"path"
	"strings"
)

type NixValue interface {
	ToString() string
}

type NixInt int64

func (i NixInt) ToString() string {
	return fmt.Sprintf("%d", i)
}

type NixFloat float64

func (f NixFloat) ToString() string {
	return fmt.Sprintf("%.6g", f)
}

type NixBool bool

func (b NixBool) ToString() string {
	if b {
		return "true"
	} else {
		return "false"
	}
}

// differentiate from the default nil
type NixNull struct{}

func (n *NixNull) ToString() string {
	return "null"
}

type NixList []*Expression

func (l NixList) ToString() string {
	last := len(l) + 1
	parts := make([]string, last+1)
	parts[0], parts[last] = "[", "]"
	for i, x := range l {
		parts[i+1] = x.ToString()
	}
	return strings.Join(parts, " ")
}
func (list NixList) Concat(newList NixList) NixList {
	return append(list, newList...)
}

type NixSet map[Sym]*Expression

func (s NixSet) ToString() string {
	last := len(s) + 1
	parts := make([]string, last+1)
	parts[0], parts[last] = "{", "}"
	i := 1
	for sym, x := range s {
		parts[i] = fmt.Sprintf("%s = %s;", sym, x.ToString())
		i++
	}
	return strings.Join(parts, " ")
}
func (set NixSet) Bind1(sym Sym, x *Expression) {
	if _, ok := set[sym]; ok {
		throw(fmt.Errorf("%v is already defined", sym))
	}
	set[sym] = x
}
func (set NixSet) Bind(syms []Sym, x *Expression) {
	last := len(syms) - 1
	for _, sym := range syms[:last] {
		if subset, ok := set[sym]; ok {
			set = subset.Value.(NixSet)
		} else {
			subset := NixSet{}
			set[sym] = &Expression{Value: subset}
			set = subset
		}
	}
	set.Bind1(syms[last], x)
}
func (set NixSet) Update(newSet NixSet) NixSet {
	result := make(NixSet, len(set))
	for sym, expr := range set {
		result[sym] = expr
	}
	for sym, expr := range newSet {
		result[sym] = expr
	}
	return result
}

type NixPath struct {
	Root string
	Path string
}

func (p *NixPath) ToString() string {
	return path.Join(p.Root, p.Path)
}

type NixString struct {
	Content string
	Context []*Derivation
	// string "..." is impure because it
	// contains "2.18" which is reference
	// to Nix version
	// { "2.18": "reference to Nix version" }
	Impurities map[string]string
}

func (str *NixString) ToString() string {
	return str.Content
}

type NixFunction struct {
	// TODO: position
	Arg         Sym
	HasArg      bool
	Formal      map[Sym]*p.Node
	HasFormal   bool
	HasEllipsis bool
	Body        *p.Node
	Scope       *Scope
}

func (f *NixFunction) ToString() string {
	return "«lambda»"
}

func InterpString(val NixValue) string {
	switch v := val.(type) {
	case *NixString:
		return v.ToString()
	case *NixPath:
		return v.ToString()
	default:
		panic(fmt.Errorf("can not coerce %v to a string", val))
	}
}

type NixNumber interface {
	NixInt | NixFloat
}

func calculate[T NixNumber](num1, num2 T, op p.NodeType) NixValue {
	switch op {
	case p.OpAddNode:
		return NixValue(num1 + num2)
	case p.OpReduceNode:
		return NixValue(num1 - num2)
	case p.OpMultiplyNode:
		return NixValue(num1 * num2)
	case p.OpDivideNode:
		return NixValue(num1 / num2)
	case p.OpGreaterNode:
		return NixBool(num1 > num2)
	case p.OpLessNode:
		return NixBool(num1 < num2)
	case p.OpGeqNode:
		return NixBool(num1 >= num2)
	case p.OpLeqNode:
		return NixBool(num1 <= num2)
	case p.OpEqNode:
		return NixBool(num1 == num2)
	default:
		panic(fmt.Sprintln("wrong operation"))
	}
}

func Calculate(val1 NixValue, val2 NixValue, op p.NodeType) NixValue {
	int1, ok1 := val1.(NixInt)
	int2, ok2 := val2.(NixInt)
	if ok1 && ok2 {
		return calculate(int1, int2, op)
	} else {
		var float1, float2 NixFloat
		var ok1_, ok2_ bool
		if ok1 {
			float1 = NixFloat(int1)
			ok1_ = true
		} else {
			float1, ok1_ = val1.(NixFloat)
		}
		if ok2 {
			float2 = NixFloat(int2)
			ok2_ = true
		} else {
			float2, ok2_ = val2.(NixFloat)
		}
		if ok1_ && ok2_ {
			return calculate(float1, float2, op)
		} else {
			panic(fmt.Sprintln("Cannot perform calculation"))
		}
	}
}
