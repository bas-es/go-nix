package main

import (
	"bufio"
	"fmt"
	"os"
	"io"

	"github.com/alecthomas/kingpin"
	"github.com/orivej/go-nix/pkg/eval"
	"github.com/orivej/go-nix/pkg/parser"
)

var (
	replCmd = kingpin.Command("repl", "Read, evaluate and print loop")
)

var replMain = register("repl", func() {
	in := bufio.NewReader(os.Stdin)
	for {
		if _, err := os.Stdout.WriteString("repl> "); err != nil {
			panic(err)
		}
		line, err := in.ReadBytes('\n')
		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}
		pr, err := parser.ParseString(string(line))
		if err != nil {
			panic(err)
		}
		fmt.Println(eval.ParseResult(pr).Print(1))
	}
})
