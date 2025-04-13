package eval

import (
	"fmt"
	"strconv"
	"strings"

	p "github.com/bas-es/go-nix/pkg/parser"
	"github.com/orivej/e"
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
}

func (x *Expression) WithNode(n *p.Node) *Expression {
	y := *x
	y.Node = n
	y.Value = nil
	y.Lower = nil
	return &y
}

func (x *Expression) WithScoped(n *p.Node, scope *Scope) *Expression {
	y := *x
	y.Scope = scope
	y.Node = n
	y.Value = nil
	y.Lower = nil
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
					set.Bind1(Intern(y.attrString()), y)
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
			set := AssertType[NixSet](val)
			if y, ok := set[sym]; ok {
				expr = y
			} else if or != nil {
				expr = or
				break
			} else {
				throw(fmt.Errorf("%v does not contain %v", y, sym))
			}
		}
		x.Lower = expr

	case p.WithNode:
		attrs := AssertType[NixSet](x.WithNode(n.Nodes[0]).Eval())
		scope := x.Scope.Subscope(attrs, true)
		x.Lower = x.WithScoped(n.Nodes[1], scope)

	case p.IfNode:
		cond := AssertType[NixBool](x.WithNode(n.Nodes[0]).Eval())
		if cond {
			x.Lower = x.WithNode(n.Nodes[1])
		} else {
			x.Lower = x.WithNode(n.Nodes[2])
		}

	case p.FunctionNode:
		fn := new(NixExprLambda)
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
		fn.Expression = x
		x.Value = fn

	case p.ApplyNode:
		arg0 := AssertType[NixLambda](x.WithNode(n.Nodes[0]).Eval())
		arg := x.WithNode(n.Nodes[1])
		x.Value = arg0.Apply(arg)

	case p.OpAddNode:
		add1 := x.WithNode(n.Nodes[0]).Eval()
		add2 := x.WithNode(n.Nodes[1]).Eval()
		if str1, ok := add1.(*NixString); ok {
			if str2, ok := add2.(*NixString); ok {
				x.Value = str1.Concat(str2)
				return
			}
		}
		x.Value = NumCalc(add1, add2, p.OpAddNode)

	case p.OpConcatNode:
		list1 := AssertType[NixList](x.WithNode(n.Nodes[0]).Eval())
		list2 := AssertType[NixList](x.WithNode(n.Nodes[1]).Eval())
		x.Value = list1.Concat(list2)

	case p.OpUpdateNode:
		set1 := AssertType[NixSet](x.WithNode(n.Nodes[0]).Eval())
		set2 := AssertType[NixSet](x.WithNode(n.Nodes[1]).Eval())
		x.Value = set1.Update(set2)

	case p.OpReduceNode, p.OpMultiplyNode, p.OpDivideNode, p.OpGreaterNode, p.OpLessNode, p.OpGeqNode, p.OpLeqNode:
		num1 := x.WithNode(n.Nodes[0]).Eval()
		num2 := x.WithNode(n.Nodes[1]).Eval()
		x.Value = NumCalc(num1, num2, nt)

	case p.OpNegateNode:
		val := x.WithNode(n.Nodes[0]).Eval()
		if i, ok := val.(NixInt); ok {
			x.Value = NixInt(-i)
		} else if f, ok := val.(NixFloat); ok {
			x.Value = NixFloat(-f)
		} else {
			panic(fmt.Sprintln("cannot give the negative form, not a number"))
		}

	case p.OpAndNode, p.OpOrNode, p.OpImplNode:
		b1 := x.WithNode(n.Nodes[0]).Eval()
		b2 := x.WithNode(n.Nodes[1]).Eval()
		x.Value = BinCalc(b1, b2, nt)

	case p.OpNotNode:
		b := AssertType[NixBool](x.WithNode(n.Nodes[0]).Eval())
		x.Value = NixBool(!b)

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
	x := Expression{Parser: pr, Node: pr.Result, Scope: DefaultScope}
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
