package eval

import (
	p "github.com/orivej/go-nix/pkg/parser"
	"sort"
)

var BuiltinsExposed = map[string]NixValue{
	"__add":          &NixPrimop{Func: bAdd, ArgNum: 2},
	"__attrNames":    &NixPrimop{Func: bAttrNames, ArgNum: 1},
	"__attrValues":   &NixPrimop{Func: bAttrValues, ArgNum: 1},
	"__concatLists":  &NixPrimop{Func: bConcatLists, ArgNum: 1},
	"__functionArgs": &NixPrimop{Func: bFunctionArgs, ArgNum: 1},
	"__tail":         &NixPrimop{Func: bTail, ArgNum: 1},
	"false":          NixBool(false),
	"null":           &NixNull{},
	"throw":          &NixPrimop{Func: bThrow, ArgNum: 1},
	"toString":       &NixPrimop{Func: bToString, ArgNum: 1},
	"true":           NixBool(true),
}

var BuiltinsInSet = map[string]NixValue{
	"add":          &NixPrimop{Func: bAdd, ArgNum: 2},
	"attrNames":    &NixPrimop{Func: bAttrNames, ArgNum: 1},
	"attrValues":   &NixPrimop{Func: bAttrValues, ArgNum: 1},
	"concatLists":  &NixPrimop{Func: bConcatLists, ArgNum: 1},
	"false":        NixBool(false),
	"functionArgs": &NixPrimop{Func: bFunctionArgs, ArgNum: 1},
	"null":         &NixNull{},
	"tail":         &NixPrimop{Func: bTail, ArgNum: 1},
	"throw":        &NixPrimop{Func: bThrow, ArgNum: 1},
	"toString":     &NixPrimop{Func: bToString, ArgNum: 1},
	"true":         NixBool(true),
}

var DefaultScope = func() *Scope {
	mainSet := make(NixSet, len(BuiltinsExposed)+1)
	builtinsSet := make(NixSet, len(BuiltinsInSet))
	for name, val := range BuiltinsExposed {
		mainSet[Intern(name)] = &Expression{Value: val}
	}
	for name, val := range BuiltinsInSet {
		builtinsSet[Intern(name)] = &Expression{Value: val}
	}
	builtinsSet[Intern("builtins")] = &Expression{Value: builtinsSet}
	mainSet[Intern("builtins")] = &Expression{Value: builtinsSet}
	return &Scope{Binds: mainSet}
}()

func bAdd(args ...*Expression) NixValue {
	return NumCalc(args[0].Eval(), args[1].Eval(), p.OpAddNode)
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

func bTail(args ...*Expression) NixValue {
	l, ok := args[0].Eval().(NixList)
	if !ok {
		panic("argument of builtins.tail is not a list")
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
