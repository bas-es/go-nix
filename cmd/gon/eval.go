package main

import (
	"fmt"

	"github.com/alecthomas/kingpin"
	"github.com/orivej/e"
	"github.com/bas-es/go-nix/pkg/eval"
	"github.com/bas-es/go-nix/pkg/parser"
)

var (
	evalCmd     = kingpin.Command("eval", "Eval Nix expression.")
	evalExprArg = evalCmd.Arg("expr", "Expression.").Required().String()
)

var evalMain = register("eval", func() {
	pr, err := parser.ParseString(*evalExprArg)
	e.Exit(err)
	fmt.Println(eval.ParseResult(pr).Print(1))
})
