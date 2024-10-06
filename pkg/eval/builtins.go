package eval

import (
	p "github.com/orivej/go-nix/pkg/parser"
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
	"concatLists":  &NixPrimop{Func: bConcatLists, ArgNum: 1},
	"false":        NixBool(false),
	"filter":       &NixPrimop{Func: bFilter, ArgNum: 2},
	"functionArgs": &NixPrimop{Func: bFunctionArgs, ArgNum: 1},
	"genList":      &NixPrimop{Func: bGenList, ArgNum: 2},
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
	"map":          &NixPrimop{Func: bMap, ArgNum: 2},
	"null":         &NixNull{},
	"partition":    &NixPrimop{Func: bPartition, ArgNum: 2},
	"tail":         &NixPrimop{Func: bTail, ArgNum: 1},
	"throw":        &NixPrimop{Func: bThrow, ArgNum: 1},
	"toString":     &NixPrimop{Func: bToString, ArgNum: 1},
	"true":         NixBool(true),
}

var DefaultScope = func() *Scope {
	mainSet := make(NixSet, len(builtinsInSet)+1)
	builtinsSet := make(NixSet, len(builtinsInSet))
	for name, val := range builtinsInSet {
		builtinsSet[Intern(name)] = &Expression{Value: val}
		if _, exists := builtinsNoUnderline[name]; exists {
			mainSet[Intern(name)] = &Expression{Value: val}
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
	f, ok := args[0].Eval().(NixValueWithApply)
	if !ok {
		panic("first argument of builtins.all is not a function or primop")
	}
	l, ok := args[1].Eval().(NixList)
	if !ok {
		panic("second argument of builtins.all is not a list")
	}
	result := true
	for _, elem := range l {
		cond, ok := f.Apply(elem).(NixBool)
		if !ok {
			panic("return value for the nth element of list is not a bool")
		}
		if !cond {
			result = false
		}
	}
	return NixBool(result)
}

func bAny(args ...*Expression) NixValue {
	f, ok := args[0].Eval().(NixValueWithApply)
	if !ok {
		panic("first argument of builtins.any is not a function or primop")
	}
	l, ok := args[1].Eval().(NixList)
	if !ok {
		panic("second argument of builtins.any is not a list")
	}
	result := false
	for _, elem := range l {
		cond, ok := f.Apply(elem).(NixBool)
		if !ok {
			panic("return value for the nth element of list is not a bool")
		}
		if cond {
			result = true
		}
	}
	return NixBool(result)
}

func bAttrNames(args ...*Expression) NixValue {
	set, ok := args[0].Eval().(NixSet)
	if !ok {
		panic("argument of builtins.attrNames is not a set")
	}
	keys := make([]string, 0, len(set))
	for sym, _ := range set {
		keys = append(keys, sym.String())
	}
	sort.Strings(keys)
	result := make(NixList, 0, len(set))
	for _, key := range keys {
		result = append(result, &Expression{Value: &NixString{Content: key}})
	}
	return result
}

func bAttrValues(args ...*Expression) NixValue {
	set, ok := args[0].Eval().(NixSet)

	if !ok {
		panic("argument of builtins.attrValues is not a set")
	}
	keys := make([]string, 0, len(set))
	for sym, _ := range set {
		keys = append(keys, sym.String())
	}
	sort.Strings(keys)
	result := make(NixList, 0, len(set))
	for _, key := range keys {
		result = append(result, set[Intern(key)])
	}
	return result
}

func bCatAttrs(args ...*Expression) NixValue {
	name, ok := args[0].Eval().(*NixString)
	if !ok {
		panic("first argument of builtins.catAttrs is not a string")
	}
	sym := Intern(name.Content)
	l, ok := args[1].Eval().(NixList)
	if !ok {
		panic("second argument of builtins.catAttrs is not a list")
	}
	result := make(NixList, 0, len(l))
	for _, expr := range l {
		set, ok := expr.Eval().(NixSet)
		if !ok {
			panic("item of list supplied to builtins.catAttrs is not a set")
		}
		result = append(result, set[sym])
	}
	return result
}

func bConcatLists(args ...*Expression) NixValue {
	list, ok := args[0].Eval().(NixList)
	if !ok {
		panic("argument of builtins.attrValues is not a set")
	}
	result := make(NixList, 0)
	for _, expr := range list {
		l, ok := expr.Eval().(NixList)
		if !ok {
			panic("child item of list is not list")
		}
		result = append(result, l...)
	}
	return result
}

func bFilter(args ...*Expression) NixValue {
	f, ok := args[0].Eval().(NixValueWithApply)
	if !ok {
		panic("first argument of builtins.filter is not a function or primop")
	}
	l, ok := args[1].Eval().(NixList)
	if !ok {
		panic("second argument of builtins.filter is not a list")
	}
	result := make(NixList, 0, len(l))
	for _, elem := range l {
		cond, ok := f.Apply(elem).(NixBool)
		if !ok {
			panic("return value for the nth element of list is not a bool")
		}
		if cond {
			result = append(result, elem)
		}
	}
	return result
}

func bFunctionArgs(args ...*Expression) NixValue {
	f, ok := args[0].Eval().(*NixFunction)
	if !ok {
		panic("argument of builtins.throw is not a function")
	}
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
	f, ok := args[0].Eval().(NixValueWithApply)
	if !ok {
		panic("first argument of builtins.genList is not a function or a primop")
	}
	num, ok := args[1].Eval().(NixInt)
	if !ok {
		panic("second argument of builtins.genList is not an integer")
	}
	result := make(NixList, 0, num)
	for i := 0; i < int(num); i++ {
		val := f.Apply(&Expression{Value: NixInt(i)})
		result = append(result, &Expression{Value: val})
	}
	return result
}

func bHead(args ...*Expression) NixValue {
	l, ok := args[0].Eval().(NixList)
	if !ok {
		panic("argument of builtins.head is not a list")
	}
	if len(l) == 0 {
		panic("argument of builtins.tail is an empty list")
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
	_, ok := args[0].Eval().(NixValueWithApply)
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

func bMap(args ...*Expression) NixValue {
	f, ok := args[0].Eval().(NixValueWithApply)
	if !ok {
		panic("first argument of builtins.map is not a function or primop")
	}
	l, ok := args[1].Eval().(NixList)
	if !ok {
		panic("second argument of builtins.map is not a list")
	}
	result := make(NixList, len(l))
	for n, elem := range l {
		result[n] = &Expression{Value: f.Apply(elem)}
	}
	return result
}

func bPartition(args ...*Expression) NixValue {
	f, ok := args[0].Eval().(NixValueWithApply)
	if !ok {
		panic("first argument of builtins.partition not a function or primop")
	}
	l, ok := args[1].Eval().(NixList)
	if !ok {
		panic("second argument of builtins.partition is not a list")
	}
	right := make(NixList, 0, len(l))
	wrong := make(NixList, 0, len(l))
	for _, elem := range l {
		if b, ok := f.Apply(elem).(NixBool); ok {
			if b {
				right = append(right, elem)
			} else {
				wrong = append(wrong, elem)
			}
		} else {
			panic("the return value of function in builtins.partition")
		}
	}
	result := make(NixSet, 2)
	result[Intern("right")] = &Expression{Value: right}
	result[Intern("wrong")] = &Expression{Value: wrong}
	return result
}

func bTail(args ...*Expression) NixValue {
	l, ok := args[0].Eval().(NixList)
	if !ok {
		panic("argument of builtins.tail is not a list")
	}
	if len(l) == 0 {
		panic("argument of builtins.tail is an empty list")
	}
	return l[1:]
}

func bToString(args ...*Expression) NixValue {
	expr, ok := args[0].Eval().(NixValueWithToString)
	if !ok {
		panic("cannot coerce to string")
	}
	return &NixString{Content: expr.ToString()}
}

func bThrow(args ...*Expression) NixValue {
	str, ok := args[0].Eval().(*NixString)
	if !ok {
		panic("argument of builtins.throw is not a string")
	}
	panic(str.Content)
}
