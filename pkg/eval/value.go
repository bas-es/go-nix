package eval

// TODO: InterpString

import (
	"fmt"
	p "github.com/orivej/go-nix/pkg/parser"
	"path"
	"strings"
)

type NixValue interface {
	Print(recurse int) string
	Compare(val NixValue) bool
}

type NixValueWithToString interface {
	NixValue
	ToString() string
}

type NixInt int64

func (i NixInt) Print(recurse int) string {
	return fmt.Sprintf("%d", i)
}

func (i NixInt) ToString() string {
	return fmt.Sprintf("%d", i)
}

func (i NixInt) Compare(val NixValue) bool {
	if i_, ok := val.(NixInt); ok {
		return i == i_
	} else if f_, ok := val.(NixFloat); ok {
		return NixFloat(i).Compare(f_)
	} else {
		return false
	}
}

type NixFloat float64

func (f NixFloat) Print(recurse int) string {
	return fmt.Sprintf("%.6g", f)
}

func (f NixFloat) ToString() string {
	return fmt.Sprintf("%.6f", f)
}

func (f NixFloat) Compare(val NixValue) bool {
	if f_, ok := val.(NixFloat); ok {
		return f.ToString() == f_.ToString()
	} else if i_, ok := val.(NixInt); ok {
		return NixFloat(i_).Compare(f)
	} else {
		return false
	}
}

type NixBool bool

func (b NixBool) Print(recurse int) string {
	if b {
		return "true"
	} else {
		return "false"
	}
}

func (b NixBool) ToString() string {
	if b {
		return "1"
	} else {
		return ""
	}
}

func (b NixBool) Compare(val NixValue) bool {
	if b_, ok := val.(NixBool); ok {
		return b == b_
	} else {
		return false
	}
}

// differentiate from the default nil
type NixNull struct{}

func (n *NixNull) Print(recurse int) string {
	return "null"
}

func (n *NixNull) ToString() string {
	return ""
}

func (n *NixNull) Compare(val NixValue) bool {
	if _, ok := val.(*NixNull); ok {
		return true
	} else {
		return false
	}
}

type NixList []*Expression

func (l NixList) Print(recurse int) string {
	if recurse == 0 {
		return "[ ... ]"
	} else {
		last := len(l) + 1
		parts := make([]string, last+1)
		parts[0], parts[last] = "[", "]"
		for i, x := range l {
			parts[i+1] = x.Eval().Print(recurse - 1)
		}
		return strings.Join(parts, " ")
	}
}

func (l NixList) ToString() string {
	var result string
	for n, elem := range l {
		ts, ok := elem.Eval().(NixValueWithToString)
		if !ok {
			panic("cannot convert list element to string")
		}
		if n == 0 {
			result = ts.ToString()
		} else {
			result += " " + ts.ToString()
		}
	}
	return result
}

func (list NixList) Concat(newList NixList) NixList {
	return append(list, newList...)
}

func (l NixList) Compare(val NixValue) bool {
	if l_, ok := val.(NixList); ok {
		if len(l) != len(l_) {
			return false
		}
		for i, v := range l {
			if !l_[i].Eval().Compare(v.Eval()) {
				return false
			}
		}
		return true
	} else {
		return false
	}
}

type NixSet map[Sym]*Expression

func (s NixSet) Print(recurse int) string {
	if recurse == 0 {
		return "{ ... }"
	} else {
		last := len(s) + 1
		parts := make([]string, last+1)
		parts[0], parts[last] = "{", "}"
		i := 1
		for sym, x := range s {
			parts[i] = fmt.Sprintf("%s = %s;", sym, x.Eval().Print(recurse-1))
			i++
		}
		return strings.Join(parts, " ")
	}
}

func (s NixSet) ToString() string {
	if toFuncExpr, exists := s[Intern("__toString")]; exists {
		toFunc, ok := toFuncExpr.Eval().(NixLambda)
		if !ok {
			panic(fmt.Sprintln("value of __toString attribute is not a function or primop"))
		}
		sExpr := &Expression{Value: s}
		ts, ok := toFunc.Apply(sExpr).(NixValueWithToString)
		if !ok {
			panic(fmt.Sprintln("cannot convert output of __toString to string"))
		}
		return ts.ToString()
	} else if outPath, exists := s[Intern("outPath")]; exists {
		if ts, ok := outPath.Eval().(NixValueWithToString); ok {
			return ts.ToString()
		}
	}
	panic(fmt.Sprintln("unable to convert set to string"))
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

func (s NixSet) Compare(val NixValue) bool {
	if s_, ok := val.(NixSet); ok {
		if len(s) != len(s_) {
			return false
		}
		for i, v := range s {
			if !s_[i].Eval().Compare(v.Eval()) {
				return false
			}
		}
		return true
	} else {
		return false
	}
}

type NixPath struct {
	Root string
	Path string
}

func (p *NixPath) Print(recurse int) string {
	return path.Join(p.Root, p.Path)
}

func (p *NixPath) ToString() string {
	return path.Join(p.Root, p.Path)
}

func (p *NixPath) Compare(val NixValue) bool {
	if p_, ok := val.(*NixPath); ok {
		return p.ToString() == p_.ToString()
	} else {
		return false
	}
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

func (str *NixString) Print(recurse int) string {
	return `"` + strings.ReplaceAll(str.Content, "\n", `\n`) + `"`
}

func (str *NixString) ToString() string {
	return str.Content
}

func (str *NixString) Concat(newStr *NixString) *NixString {
	impurities := make(map[string]string, len(str.Impurities))
	for name, reason := range str.Impurities {
		impurities[name] = reason
	}
	for name, reason := range newStr.Impurities {
		impurities[name] = reason
	}
	context := append(str.Context, newStr.Context...)
	return &NixString{Content: str.Content + newStr.Content, Context: context, Impurities: impurities}
}

func (str *NixString) Compare(val NixValue) bool {
	if str_, ok := val.(*NixString); ok {
		return str.Content == str_.Content
	} else {
		return false
	}
}

type NixLambda interface {
	NixValue
	Apply(*Expression) NixValue
}

type NixExprLambda struct {
	// TODO: position
	Arg         Sym
	HasArg      bool
	Formal      map[Sym]*p.Node
	HasFormal   bool
	HasEllipsis bool
	Body        *p.Node
	Expression  *Expression
}

func (f *NixExprLambda) Print(recurse int) string {
	return "«lambda»"
}

func (f *NixExprLambda) Compare(val NixValue) bool {
	return false
}

func (f *NixExprLambda) Apply(expr *Expression) NixValue {
	set := make(NixSet, 1)
	fnExpr := f.Expression
	scope := fnExpr.Scope.Subscope(set, false)
	// TODO: order wrong?
	if f.HasArg {
		set[f.Arg] = expr
	}
	if f.HasFormal {
		argSet, ok := expr.Eval().(NixSet)
		if !ok {
			panic(fmt.Sprintln("calling a function with formal but argument is not a set"))
		}
		for sym, exprNode := range f.Formal {
			if f.HasArg && sym == f.Arg {
				panic(fmt.Sprintln("duplicate formal and function argument"))
			}
			if exprNode != nil {
				set[sym] = fnExpr.WithScoped(exprNode, scope)
			}
		}
		for sym, expr := range argSet {
			if _, exists := f.Formal[sym]; exists {
				set[sym] = expr
			} else if !f.HasEllipsis {
				panic(fmt.Sprintln("set has more than enough formals to call a function"))
			}
		}
	}
	// TODO: Not so lazy?
	return fnExpr.WithScoped(f.Body, scope).Eval()
}

// Builtin functions
type NixPrimop struct {
	Func   func(...*Expression) NixValue
	Doc    string
	Name   string
	ArgNum int
}

func (p *NixPrimop) Print(recurse int) string {
	return fmt.Sprintf("«primop %s»", p.Name)
}

func (p *NixPrimop) Compare(val NixValue) bool {
	return false
}

func (p *NixPrimop) Apply(expr *Expression) NixValue {
	if p.ArgNum == 1 {
		return p.Func(expr)
	} else {
		queue := make([]*Expression, 0, p.ArgNum)
		queue = append(queue, expr)
		return &NixPartialPrimop{Primop: p, ArgQueue: queue}
	}
}

type NixPartialPrimop struct {
	Primop   *NixPrimop
	ArgQueue []*Expression
}

func (pp *NixPartialPrimop) Print(recurse int) string {
	return fmt.Sprintf("«primop %s, with %d/%d argument»", pp.Primop.Name, len(pp.ArgQueue), pp.Primop.ArgNum)
}

func (pp *NixPartialPrimop) Compare(val NixValue) bool {
	return false
}

func (pp *NixPartialPrimop) Apply(expr *Expression) NixValue {
	ppNew := *pp
	ppNew.ArgQueue = append(pp.ArgQueue, expr)
	if len(ppNew.ArgQueue) == ppNew.Primop.ArgNum {
		return ppNew.Primop.Func(ppNew.ArgQueue...)
	} else {
		return &ppNew
	}
}

func InterpString(val NixValue) string {
	switch v := val.(type) {
	case *NixString:
		return v.ToString()
	case *NixPath:
		return v.ToString()
	case NixSet:
		return v.ToString()
	default:
		panic(fmt.Errorf("can not coerce %v to a string", val))
	}
}

type NixNumber interface {
	NixInt | NixFloat
}

func numCalc[T NixNumber](num1, num2 T, op p.NodeType) NixValue {
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

func NumCalc(val1 NixValue, val2 NixValue, op p.NodeType) NixValue {
	int1, ok1 := val1.(NixInt)
	int2, ok2 := val2.(NixInt)
	if ok1 && ok2 {
		return numCalc(int1, int2, op)
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
			return numCalc(float1, float2, op)
		} else {
			panic(fmt.Sprintln("Cannot perform calculation"))
		}
	}
}

func BinCalc(val1 NixValue, val2 NixValue, op p.NodeType) NixBool {
	b1 := AssertType[NixBool](val1)
	b2 := AssertType[NixBool](val2)
	switch op {
	case p.OpAndNode:
		return NixBool(b1 && b2)
	case p.OpOrNode:
		return NixBool(b1 || b2)
	case p.OpImplNode:
		return NixBool(!b1 || b2)
	default:
		panic(fmt.Sprintln("wrong operation"))
	}
}
