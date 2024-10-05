package eval

import (
	"fmt"
	"github.com/orivej/e"
	p "github.com/orivej/go-nix/pkg/parser"
	"strconv"
	"strings"
)

type Scope struct {
	Binds   NixSet
	LowPrio bool
	Parent  *Scope
}

func (scope *Scope) Subscope(binds NixSet, lowPrio bool) *Scope {
	return &Scope{Binds: binds, LowPrio: lowPrio, Parent: scope}
}

type Expression struct {
	Value  NixValue
	Lower  *Expression
	Scope  *Scope
	Parser *p.Parser
	Node   *p.Node
	Sym    Sym
}

// TODO: recursive :p logic should be here
func (x *Expression) ToString() string {
	if x.Value == nil {
		return "..."
	} else {
		return x.Value.ToString()
	}
}

func (x *Expression) WithNode(n *p.Node) *Expression {
	y := *x
	y.Node = n
	return &y
}

func (x *Expression) WithScoped(n *p.Node, scope *Scope) *Expression {
	y := *x
	y.Scope = scope
	y.Node = n
	return &y
}

func (x *Expression) Eval() NixValue {
	// TODO: evaluating undefined identifier may succeeded if
	// It's not needed for result. Is this ideal?
	// e.g. `let a = b; in 1`, `rec { a = b; }`
	expr := x
	for expr.Value == nil {
		if expr.Lower != nil {
			expr = expr.Lower
			// lower may already have value?
		} else {
			expr.resolve()
		}
	}
	return expr.Value
}

func (x *Expression) tokenString(i int) string {
	return x.Parser.TokenString(x.Node.Tokens[i])
}

func (x *Expression) resolve() {
	if x.Value != nil || x.Lower != nil {
		return
	}
	pr := x.Parser
	n := x.Node
	nt := x.Node.Type
	if x.Sym != 0 {
		nt = p.IDNode
	}
	switch nt {
	default:
		panic(fmt.Sprintln("unsupported node type:", n.Type))
	case p.URINode:
		x.Value = &NixString{Content: x.tokenString(0)}
	case p.PathNode:
		// TODO: Absolute path/Flake related path
		x.Value = &NixPath{Root: "/", Path: x.tokenString(0)}
	case p.FloatNode:
		val, err := strconv.ParseFloat(x.tokenString(0), 64)
		noerr(err)
		x.Value = NixFloat(val)
	case p.IntNode:
		val, err := strconv.ParseInt(x.tokenString(0), 10, 64)
		noerr(err)
		x.Value = NixInt(val)

	case p.StringNode, p.IStringNode:
		parts := make([]string, len(n.Nodes))
		for i, c := range n.Nodes {
			switch c.Type {
			default:
				panic(fmt.Sprintln("unsupported string part type:", c.Type))
			case p.TextNode:
				parts[i] = pr.TokenString(c.Tokens[0])
			case p.InterpNode:
				y := x.WithNode(c.Nodes[0])
				parts[i] = InterpString(y.Eval())
			}
		}
		// TODO: Add InterpNode to context
		x.Value = &NixString{Content: strings.Join(parts, "")}

	case p.IDNode:
		currentScope := x.Scope
		sym := Intern(x.tokenString(0))
		lowPrio := false
		for {
			if currentScope == nil {
				if lowPrio {
					panic(fmt.Sprintln("variable of sym not found:", sym))
				} else {
					currentScope = x.Scope
					lowPrio = true
					continue
				}
			}
			if currentScope.LowPrio == lowPrio {
				if y, exists := currentScope.Binds[sym]; exists {
					if y == x {
						panic(fmt.Sprintln("infinite recursion encountered"))
					} else {
						x.Lower = y
						break
					}
				}
			}
			currentScope = currentScope.Parent
		}

	case p.ParensNode:
		x.Lower = x.WithNode(n.Nodes[0])

	case p.ListNode:
		parts := make(NixList, len(n.Nodes))
		for i, c := range n.Nodes {
			parts[i] = x.WithNode(c)
		}
		x.Value = parts

	case p.SetNode, p.RecSetNode, p.LetNode:
		var bindNodes []*p.Node
		if nt == p.LetNode {
			bindNodes = n.Nodes[0].Nodes
		} else {
			bindNodes = n.Nodes
		}
		set := make(NixSet, len(bindNodes)) // Inheriting makes it larger than this.
		scope := x.Scope
		if nt == p.RecSetNode || nt == p.LetNode {
			scope = scope.Subscope(set, false)
		}
		for _, c := range bindNodes {
			switch c.Type {
			case p.BindNode:
				attrpath := x.WithScoped(c.Nodes[0], scope).evalAttrpath()
				set.Bind(attrpath, x.WithScoped(c.Nodes[1], scope))
			case p.InheritNode:
				for _, interpid := range c.Nodes[0].Nodes {
					y := x.WithNode(interpid)
					y.Sym = Intern(y.attrString())
					set.Bind1(y.Sym, y)
				}
			case p.InheritFromNode:
				// This is not as lazy as it can be.
				from := x.WithScoped(c.Nodes[0], scope)
				for _, interpid := range c.Nodes[1].Nodes {
					// let c = { a = 1; }; in let inherit (c) b; in 1
					// should not success in sense, but nix does success
					// so we don't complain about it
					sym := Intern(x.WithNode(interpid).attrString())
					set.Bind1(sym, from.Select1(sym))
				}
			}
		}
		if nt == p.LetNode {
			x.Lower = x.WithScoped(n.Nodes[1], scope)
		} else {
			x.Value = set
		}

	case p.SelectNode, p.SelectOrNode:
		attrpath := x.WithNode(n.Nodes[1]).evalAttrpath()
		var or *Expression
		if nt == p.SelectOrNode {
			or = x.WithNode(n.Nodes[2])
		}
		expr := x.WithNode(n.Nodes[0])
		for _, sym := range attrpath {
			val := expr.Eval()
			if set, ok := val.(NixSet); ok {
				if y, ok := set[sym]; ok {
					expr = y
				} else if or != nil {
					expr = or
					break
				} else {
					throw(fmt.Errorf("%v does not contain %v", y, sym))
				}
			} else {
				throw(fmt.Errorf("%v is not a set", val))
			}
		}
		x.Lower = expr

	case p.WithNode:
		attrs, ok := x.WithNode(n.Nodes[0]).Eval().(NixSet)
		if !ok {
			// TODO: printing attrs here is wrong
			panic(fmt.Sprintln("argument of with is not a set:", attrs))
		}
		scope := x.Scope.Subscope(attrs, true)
		x.Lower = x.WithScoped(n.Nodes[1], scope)

	case p.IfNode:
		cond, ok := x.WithNode(n.Nodes[0]).Eval().(NixBool)
		if !ok {
			// TODO: printing attrs here is wrong
			panic(fmt.Sprintln("if condition does not evaluate to a boolean value"))
		}
		if cond {
			x.Lower = x.WithNode(n.Nodes[1])
		} else {
			x.Lower = x.WithNode(n.Nodes[2])
		}

	case p.FunctionNode:
		fn := new(NixFunction)
		for c, node := range n.Nodes {
			if node.Type == p.ArgSetNode {
				fn.Formal = make(map[Sym]*p.Node, len(node.Nodes))
				fn.HasFormal = true
				for _, arg := range node.Nodes {
					if len(arg.Nodes) == 0 {
						fn.HasEllipsis = true
						continue
					}
					sym := Intern(x.WithNode(arg.Nodes[0]).tokenString(0))
					var exprNode *p.Node
					if len(arg.Nodes) == 2 {
						exprNode = arg.Nodes[1]
					}
					fn.Formal[sym] = exprNode
				}
			} else if c >= 1 {
				fn.Body = node
			} else {
				fn.Arg = Intern(x.WithNode(node).tokenString(0))
				fn.HasArg = true
			}
		}
		fn.Scope = x.Scope
		x.Value = fn

	case p.ApplyNode:
		fn, ok := x.WithNode(n.Nodes[0]).Eval().(*NixFunction)
		if !ok {
			panic(fmt.Sprintln("attempt to call something which is not a function"))
		}
		arg := x.WithNode(n.Nodes[1])
		var out *Expression
		set := make(NixSet, 1)
		// TODO: Clearly explain why we need scope from function?
		// Because body isn't converted to an expression?
		scope := fn.Scope.Subscope(set, false)
		// TODO: order wrong?
		if fn.HasArg {
			set[fn.Arg] = arg
		}
		if fn.HasFormal {
			argSet, ok := arg.Eval().(NixSet)
			if !ok {
				panic(fmt.Sprintln("calling a function with formal but argument is not a set"))
			}
			for sym, exprNode := range fn.Formal {
				if fn.HasArg && sym == fn.Arg {
					panic(fmt.Sprintln("duplicate formal function argument"))
				}
				if exprNode != nil {
					set[sym] = x.WithScoped(exprNode, scope)
				}
			}
			for sym, expr := range argSet {
				if _, exists := fn.Formal[sym]; exists {
					set[sym] = expr
				} else if !fn.HasEllipsis {
					panic(fmt.Sprintln("set has more than enough formals to call a function"))
				}
			}
		}
		out = x.WithScoped(fn.Body, scope)
		x.Lower = out

	case p.OpAddNode:
		add1 := x.WithNode(n.Nodes[0]).Eval()
		add2 := x.WithNode(n.Nodes[1]).Eval()
		if str1, ok := add1.(*NixString); ok {
			if str2, ok := add2.(*NixString); ok {
				x.Value = str1.Concat(str2)
				return
			}
		}
		x.Value = Calculate(add1, add2, p.OpAddNode)

	case p.OpConcatNode:
		add1 := x.WithNode(n.Nodes[0]).Eval()
		add2 := x.WithNode(n.Nodes[1]).Eval()
		if list1, ok := add1.(NixList); ok {
			if list2, ok := add2.(NixList); ok {
				x.Value = list1.Concat(list2)
				return
			}
		}
		panic(fmt.Sprintln("cannot concat two values in one or two of which is not list"))

	case p.OpUpdateNode:
		add1 := x.WithNode(n.Nodes[0]).Eval()
		add2 := x.WithNode(n.Nodes[1]).Eval()
		if set1, ok := add1.(NixSet); ok {
			if set2, ok := add2.(NixSet); ok {
				x.Value = set1.Update(set2)
				return
			}
		}
		panic(fmt.Sprintln("cannot update two values in one or two of which is not set"))

	case p.OpReduceNode, p.OpMultiplyNode, p.OpDivideNode, p.OpGreaterNode, p.OpLessNode, p.OpGeqNode, p.OpLeqNode:
		num1 := x.WithNode(n.Nodes[0]).Eval()
		num2 := x.WithNode(n.Nodes[1]).Eval()
		x.Value = Calculate(num1, num2, nt)

	case p.OpEqNode, p.OpNeqNode:
		val1 := x.WithNode(n.Nodes[0]).Eval()
		val2 := x.WithNode(n.Nodes[1]).Eval()
		result := val1.Compare(val2)
		if nt == p.OpEqNode {
			x.Value = NixBool(result)
		} else {
			x.Value = NixBool(!result)
		}
	}

	return
}

func (x *Expression) Select1(sym Sym) *Expression {
	return x.Eval().(NixSet)[sym]
}

func (x *Expression) evalAttrpath() []Sym {
	attrs := make([]Sym, len(x.Node.Nodes))
	for i, c := range x.Node.Nodes {
		y := x.WithNode(c)
		switch c.Type {
		case p.IDNode:
			attrs[i] = Intern(y.tokenString(0))
		case p.StringNode:
			attrs[i] = Intern(InterpString(y.Eval()))
		case p.InterpNode:
			attrs[i] = Intern(InterpString(y.WithNode(y.Node.Nodes[0]).Eval()))
		default:
			panic(fmt.Errorf("unsupported attrpath node %v", c.Type))
		}
	}
	return attrs
}

func (x *Expression) attrString() string {
	switch x.Node.Type {
	case p.IDNode:
		return x.tokenString(0)
	case p.StringNode:
		return x.Eval().(*NixString).Content
	case p.InterpNode:
		return InterpString(x.WithNode(x.Node.Nodes[0]).Eval())
	default:
		panic(fmt.Errorf("unsupported attr type %v", x.Node.Type))
	}
}

func ParseResult(pr *p.Parser) NixValue {
	x := Expression{Parser: pr, Node: pr.Result}
	return x.Eval()
}

func throw(err error) {
	if err != nil {
		panic(err)
	}
}

func noerr(err error) {
	e.Exit(err)
}
