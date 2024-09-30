package main

import (
	"testing"

	"github.com/alecthomas/assert"
	"github.com/orivej/go-nix/nix/eval"
	"github.com/orivej/go-nix/nix/parser"
)

func TestEval(t *testing.T) {
	for _, test := range [][2]string{
		{`{ "abc" = 1; }`, `{ abc = ...; }`},
		{`{ abc = 1; }`, `{ abc = ...; }`},
		{`{ a = 1; }.a`, `1`},
		{`{ a."b".${"c"} = 1; }."${"a"}".${"b"}.c`, `1`},
		{`(rec { a = 2; b = a; }).b`, `2`},
		{`(rec { a.b.c = 2; b = a; }).b`, `{ b = { c = ...; }; }`},
		{`rec { a = rec { d = b; }; b = 3; }.a.d`, `3`},
	} {
		pr, err := parser.ParseString(test[0])
		assert.NoError(t, err)
		s := eval.ValueString(eval.ParseResult(pr))
		assert.Equal(t, test[1], s)
	}
}
