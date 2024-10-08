package eval

import (
	p "github.com/orivej/go-nix/pkg/parser"
	"math"
	"regexp"
	"sort"
)

var builtinsNoUnderline = map[string]bool{
	"false":    true,
	"isNull":   true,
	"map":      true,
	"null":     true,
	"throw":    true,
	"toString": true,
	"true":     true,
}

var builtinsInSet = map[string]NixValue{
	"add":          &NixPrimop{Func: bAdd, ArgNum: 2},
	"all":          &NixPrimop{Func: bAll, ArgNum: 2},
	"any":          &NixPrimop{Func: bAny, ArgNum: 2},
	"attrNames":    &NixPrimop{Func: bAttrNames, ArgNum: 1},
	"attrValues":   &NixPrimop{Func: bAttrValues, ArgNum: 1},
	"catAttrs":     &NixPrimop{Func: bCatAttrs, ArgNum: 2},
	"ceil":         &NixPrimop{Func: bCeil, ArgNum: 1},
	"concatLists":  &NixPrimop{Func: bConcatLists, ArgNum: 1},
	"div":          &NixPrimop{Func: bDiv, ArgNum: 2},
	"elem":         &NixPrimop{Func: bElem, ArgNum: 2},
	"elemAt":       &NixPrimop{Func: bElemAt, ArgNum: 2},
	"false":        NixBool(false),
	"filter":       &NixPrimop{Func: bFilter, ArgNum: 2},
	"floor":        &NixPrimop{Func: bFloor, ArgNum: 1},
	"fromJSON":     &NixPrimop{Func: bFromJSON, ArgNum: 1},
	"functionArgs": &NixPrimop{Func: bFunctionArgs, ArgNum: 1},
	"genList":      &NixPrimop{Func: bGenList, ArgNum: 2},
	"getAttr":      &NixPrimop{Func: bGetAttr, ArgNum: 2},
	"groupBy":      &NixPrimop{Func: bGroupBy, ArgNum: 2},
	"head":         &NixPrimop{Func: bHead, ArgNum: 1},
	"isAttrs":      &NixPrimop{Func: bIsAttrs, ArgNum: 1},
	"isBool":       &NixPrimop{Func: bIsBool, ArgNum: 1},
	"isFloat":      &NixPrimop{Func: bIsFloat, ArgNum: 1},
	"isFunction":   &NixPrimop{Func: bIsFunction, ArgNum: 1},
	"isInt":        &NixPrimop{Func: bIsInt, ArgNum: 1},
	"isList":       &NixPrimop{Func: bIsList, ArgNum: 1},
	"isNull":       &NixPrimop{Func: bIsNull, ArgNum: 1},
	"isPath":       &NixPrimop{Func: bIsPath, ArgNum: 1},
	"isString":     &NixPrimop{Func: bIsString, ArgNum: 1},
	"length":       &NixPrimop{Func: bLength, ArgNum: 1},
	"lessThan":     &NixPrimop{Func: bLessThan, ArgNum: 2},
	"map":          &NixPrimop{Func: bMap, ArgNum: 2},
	"match":        &NixPrimop{Func: bMatch, ArgNum: 2},
	"mul":          &NixPrimop{Func: bMul, ArgNum: 2},
	"null":         &NixNull{},
	"partition":    &NixPrimop{Func: bPartition, ArgNum: 2},
	"seq":          &NixPrimop{Func: bSeq, ArgNum: 2},
	"sort":         &NixPrimop{Func: bSort, ArgNum: 2},
	"stringLength": &NixPrimop{Func: bStringLength, ArgNum: 1},
	"sub":          &NixPrimop{Func: bSub, ArgNum: 2},
	"tail":         &NixPrimop{Func: bTail, ArgNum: 1},
	"throw":        &NixPrimop{Func: bThrow, ArgNum: 1},
	"toJSON":       &NixPrimop{Func: bToJSON, ArgNum: 1},
	"toString":     &NixPrimop{Func: bToString, ArgNum: 1},
	"true":         NixBool(true),
	"typeOf":       &NixPrimop{Func: bTypeOf, ArgNum: 1},
}

var DefaultScope = func() *Scope {
	mainSet := make(NixSet, len(builtinsInSet)+1)
	builtinsSet := make(NixSet, len(builtinsInSet))
	for name, val := range builtinsInSet {
		sym := Intern(name)
		if primop, ok := val.(*NixPrimop); ok {
			primop.Sym = sym
		}
		builtinsSet[sym] = &Expression{Value: val}
		if _, exists := builtinsNoUnderline[name]; exists {
			mainSet[sym] = &Expression{Value: val}
		} else {
			mainSet[Intern("__"+name)] = &Expression{Value: val}
		}
	}
	builtinsSet[Intern("builtins")] = &Expression{Value: builtinsSet}
	mainSet[Intern("builtins")] = &Expression{Value: builtinsSet}
	return &Scope{Binds: mainSet}
}()

func bAdd(args ...*Expression) NixValue {
	return NumCalc(args[0].Eval(), args[1].Eval(), p.OpAddNode)
}

func bAll(args ...*Expression) NixValue {
	f := AssertType[NixLambda](args[0].Eval())
	l := AssertType[NixList](args[1].Eval())
	for _, elem := range l {
		cond := AssertType[NixBool](f.Apply(elem))
		if !cond {
			return NixBool(false)
		}
	}
	return NixBool(true)
}

func bAny(args ...*Expression) NixValue {
	f := AssertType[NixLambda](args[0].Eval())
	l := AssertType[NixList](args[1].Eval())
	for _, elem := range l {
		cond := AssertType[NixBool](f.Apply(elem))
		if cond {
			return NixBool(true)
		}
	}
	return NixBool(false)
}

func bAttrNames(args ...*Expression) NixValue {
	set := AssertType[NixSet](args[0].Eval())
	result := make(NixList, 0, len(set))
	for _, sym := range set.Iterator() {
		result = append(result, &Expression{Value: &NixString{Content: sym.String()}})
	}
	return result
}

func bAttrValues(args ...*Expression) NixValue {
	set := AssertType[NixSet](args[0].Eval())
	result := make(NixList, 0, len(set))
	for _, sym := range set.Iterator() {
		result = append(result, set[sym])
	}
	return result
}

func bCatAttrs(args ...*Expression) NixValue {
	name := AssertType[*NixString](args[0].Eval())
	sym := Intern(name.Content)
	l := AssertType[NixList](args[1].Eval())
	result := make(NixList, 0, len(l))
	for _, expr := range l {
		set := AssertType[NixSet](expr.Eval())
		result = append(result, set[sym])
	}
	return result
}

func bCeil(args ...*Expression) NixValue {
	fl := AssertType[NixFloat](args[0].Eval())
	return NixInt(math.Ceil(float64(fl)))
}

func bConcatLists(args ...*Expression) NixValue {
	list := AssertType[NixList](args[0].Eval())
	result := make(NixList, 0)
	for _, expr := range list {
		l := AssertType[NixList](expr.Eval())
		result = append(result, l...)
	}
	return result
}

func bDiv(args ...*Expression) NixValue {
	return NumCalc(args[0].Eval(), args[1].Eval(), p.OpDivideNode)
}

func bElem(args ...*Expression) NixValue {
	elem := AssertType[NixInt](args[0].Eval())
	list := AssertType[NixList](args[1].Eval())
	for _, expr := range list {
		if elem.Compare(expr.Eval()) {
			return NixBool(true)
		}
	}
	return NixBool(false)
}

func bElemAt(args ...*Expression) NixValue {
	list := AssertType[NixList](args[0].Eval())
	index := AssertType[NixInt](args[1].Eval())
	if int64(index) >= int64(len(list)) {
		panic("index is out of bounds of list provided")
	}
	return list[index].Eval()
}

func bFilter(args ...*Expression) NixValue {
	f := AssertType[NixLambda](args[0].Eval())
	l := AssertType[NixList](args[1].Eval())
	result := make(NixList, 0, len(l))
	for _, elem := range l {
		cond := AssertType[NixBool](f.Apply(elem))
		if cond {
			result = append(result, elem)
		}
	}
	return result
}

func bFloor(args ...*Expression) NixValue {
	fl := AssertType[NixFloat](args[0].Eval())
	return NixInt(math.Floor(float64(fl)))
}

func bFromJSON(args ...*Expression) NixValue {
	str := AssertType[*NixString](args[0].Eval())
	return ValueFromJSON(str)
}

func bFunctionArgs(args ...*Expression) NixValue {
	f := AssertType[*NixExprLambda](args[0].Eval())
	if f.HasFormal {
		result := make(NixSet, len(f.Formal))
		for sym, node := range f.Formal {
			if node != nil {
				result[sym] = &Expression{Value: NixBool(true)}
			} else {
				result[sym] = &Expression{Value: NixBool(false)}
			}
		}
		return result
	} else {
		return make(NixSet, 0)
	}
}

func bGenList(args ...*Expression) NixValue {
	f := AssertType[NixLambda](args[0].Eval())
	num := AssertType[NixInt](args[1].Eval())
	result := make(NixList, 0, num)
	for i := 0; i < int(num); i++ {
		val := f.Apply(&Expression{Value: NixInt(i)})
		result = append(result, &Expression{Value: val})
	}
	return result
}

func bGetAttr(args ...*Expression) NixValue {
	name := AssertType[*NixString](args[0].Eval())
	set := AssertType[NixSet](args[1].Eval())
	if expr, exists := set[Intern(name.Content)]; exists {
		return expr.Eval()
	} else {
		panic("set doesn't have given attribute name")
	}
}

func bGroupBy(args ...*Expression) NixValue {
	f := AssertType[NixLambda](args[0].Eval())
	l := AssertType[NixList](args[1].Eval())
	result := make(NixSet, 0)
	for _, elem := range l {
		group := AssertType[*NixString](f.Apply(elem))
		sym := Intern(group.Content)
		if expr, ok := result[sym]; ok {
			l := AssertType[NixList](expr.Value)
			result[sym].Value = append(l, elem)
		} else {
			lg := make(NixList, 0)
			lg = append(lg, elem)
			result[sym] = &Expression{Value: lg}
		}
	}
	return result
}

func bHead(args ...*Expression) NixValue {
	l := AssertType[NixList](args[0].Eval())
	if len(l) == 0 {
		panic("argument of builtins.head is an empty list")
	}
	return l[0].Eval()
}

func bIsAttrs(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(NixSet)
	return NixBool(ok)
}

func bIsBool(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(NixBool)
	return NixBool(ok)
}

func bIsFloat(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(NixFloat)
	return NixBool(ok)
}

func bIsFunction(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(NixLambda)
	return NixBool(ok)
}

func bIsInt(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(NixInt)
	return NixBool(ok)
}

func bIsList(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(NixList)
	return NixBool(ok)
}

func bIsNull(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(*NixNull)
	return NixBool(ok)
}

func bIsPath(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(*NixPath)
	return NixBool(ok)
}

func bIsString(args ...*Expression) NixValue {
	_, ok := args[0].Eval().(*NixString)
	return NixBool(ok)
}

func bLength(args ...*Expression) NixValue {
	l := AssertType[NixList](args[0].Eval())
	return NixInt(len(l))
}

func bLessThan(args ...*Expression) NixValue {
	return NumCalc(args[0].Eval(), args[1].Eval(), p.OpLessNode)
}

func bMap(args ...*Expression) NixValue {
	f := AssertType[NixLambda](args[0].Eval())
	l := AssertType[NixList](args[1].Eval())
	result := make(NixList, len(l))
	for n, elem := range l {
		result[n] = &Expression{Value: f.Apply(elem)}
	}
	return result
}

func bMatch(args ...*Expression) NixValue {
	reStr := AssertType[*NixString](args[0].Eval())
	str := AssertType[*NixString](args[1].Eval())
	re, err := regexp.CompilePOSIX(reStr.Content)
	if err != nil {
		panic(err)
	}
	re.Longest()
	matches := re.FindStringSubmatch(str.Content)
	if matches == nil || matches[0] != str.Content {
		return &NixNull{}
	}
	result := make(NixList, 0, len(matches))
	for _, match := range matches[1:] {
		result = append(result, &Expression{Value: &NixString{Content: match}})
	}
	return result
}

func bMul(args ...*Expression) NixValue {
	return NumCalc(args[0].Eval(), args[1].Eval(), p.OpMultiplyNode)
}

func bPartition(args ...*Expression) NixValue {
	f := AssertType[NixLambda](args[0].Eval())
	l := AssertType[NixList](args[1].Eval())
	right := make(NixList, 0, len(l))
	wrong := make(NixList, 0, len(l))
	for _, elem := range l {
		b := AssertType[NixBool](f.Apply(elem))
		if b {
			right = append(right, elem)
		} else {
			wrong = append(wrong, elem)
		}
	}
	result := make(NixSet, 2)
	result[Intern("right")] = &Expression{Value: right}
	result[Intern("wrong")] = &Expression{Value: wrong}
	return result
}

func bSeq(args ...*Expression) NixValue {
	args[0].Eval()
	return args[1].Eval()
}

func bSort(args ...*Expression) NixValue {
	f := AssertType[NixLambda](args[0].Eval())
	l_ := AssertType[NixList](args[1].Eval())
	l := make(NixList, len(l_))
	copy(l, l_)
	sort.SliceStable(l, func(i1, i2 int) bool {
		f1 := AssertType[NixLambda](f.Apply(l[i1]))
		b := AssertType[NixBool](f1.Apply(l[i2]))
		return bool(b)
	})
	return l
}

func bStringLength(args ...*Expression) NixValue {
	str := AssertType[*NixString](args[0].Eval())
	return NixInt(len(str.Content))
}

func bSub(args ...*Expression) NixValue {
	return NumCalc(args[0].Eval(), args[1].Eval(), p.OpReduceNode)
}

func bTail(args ...*Expression) NixValue {
	l := AssertType[NixList](args[0].Eval())
	if len(l) == 0 {
		panic("argument of builtins.tail is an empty list")
	}
	return l[1:]
}

func bToJSON(args ...*Expression) NixValue {
	return ValueToJSON(args[0].Eval())
}

func bToString(args ...*Expression) NixValue {
	expr, ok := args[0].Eval().(NixValueWithToString)
	if !ok {
		panic("cannot coerce to string")
	}
	return &NixString{Content: expr.ToString()}
}

func bThrow(args ...*Expression) NixValue {
	str := AssertType[*NixString](args[0].Eval())
	panic(str.Content)
}

func bTypeOf(args ...*Expression) NixValue {
	return &NixString{Content: PrintType(args[0].Eval())}
}
