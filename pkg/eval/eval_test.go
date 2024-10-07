package eval

import (
	"testing"

	"github.com/alecthomas/assert"
	"github.com/orivej/go-nix/pkg/parser"
)

func TestEval(t *testing.T) {
	for _, test := range [][2]string{
		{`{ "abc" = 1; }`, `{ abc = 1; }`},
		{`{ abc = 1; }`, `{ abc = 1; }`},
		{`{ a = 1; }.a`, `1`},
		{`{ a."b".${"c"} = 1; }."${"a"}".${"b"}.c`, `1`},
		{`(rec { a = 2; b = a; }).b`, `2`},
		{`(rec { a.b.c = 2; b = a; }).b`, `{ b = { ... }; }`},
		{`rec { a = rec { d = b; }; b = 3; }.a.d`, `3`},
		{`let a = 1; b = 2; in b`, `2`},
		{`let a = 1; in let b = 2; in a`, `1`},
		{`let a = 1; in { inherit a; }.a`, `1`},
		{`let a.c = 1; in let inherit (a) c; in c`, `1`},
		{`let a = 1; b = a; in b`, `1`},
		{`with { a = 1; }; a`, `1`},
		{`let a = 2; in with { a = 1; }; a`, `2`},
		{`with { a = 2; }; with { a = 1; }; a`, `1`},
		{`(a: a) 1`, `1`},
		{`({a,b,c}: b) { a = 1; b = 2; c = 3; }`, `2`},
		{`({a,b,...}: b) { a = 1; b = 2; c = 3; }`, `2`},
		{`({a,b,d?4,...}: d) { a = 1; b = 2; c = 3; }`, `4`},
		{`(({a,b?3,...}@arg: arg) { a = 1; b = 2; c = 3; }).b`, `2`},
		{`(({a,b?arg}@arg: b) { a = 2; }).a`, `2`},
		{`(a: b: a: b) 1 2 3`, `2`},
		{`(a: { a ? 2 }: a) 1 {}`, `2`},
		{`.2`, `0.2`},
		{`1.`, `1`},
		{`3.232111`, `3.23211`},
		{`3.232115`, `3.23211`},
		{`3.232116`, `3.23212`},
		{`3.232199`, `3.2322`},
		{`323119232739.`, `3.23119e+11`},
		{`{a.b = 1;} == { a = { b = 1; }; }`, `true`},
		{`2.47207e+17 == 247207427047107403`, `false`},
		{`[[1][2 2]] ++ [[3 3 3]] == [[1][2 2][3 3 3]]`, `true`},
		{`let s = { a.b = (a: a); }; in s == s`, `false`},
	} {
		pr, err := parser.ParseString(test[0])
		assert.NoError(t, err)
		s := ParseResult(pr).Print(1)
		assert.Equal(t, test[1], s)
	}
}
