package parser

//go:generate ragel -Z -G2 machine.rl

import (
	"fmt"
	"go/token"
	"io/ioutil"
)

type lexerToken struct{ sym, pos, end, prev int }

type lexResult struct {
	file     *token.File
	data     []byte
	tokens   []lexerToken
	comments []lexerToken
}

type LexPosition token.Position

func (p *LexPosition) String() string {
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

type LexerError struct {
	Pos *LexPosition
	Desc string
}

func (e *LexerError) Error() string {
	return fmt.Sprintf("%s: syntax error: %s", e.Pos.String(), e.Desc)
}

var fileset = token.NewFileSet()

func newLexResult(path string, size int) *lexResult {
	if path == "" {
		path = "«string»"
	}
	return &lexResult{file: fileset.AddFile(path, -1, size)}
}

func (r *lexResult) TokenPos(i int) *LexPosition {
	p := LexPosition(r.file.Position(r.file.Pos(r.tokens[i].pos)))
	return &p
}

func (r *lexResult) TokenBytes(i int) []byte {
	tok := r.tokens[i]
	return r.data[tok.pos:tok.end]
}

func (r *lexResult) TokenString(i int) string {
	return string(r.TokenBytes(i))
}

func (r *lexResult) TokenSymString(i int) string {
	sym := r.tokens[i].sym
	if sym >= yyPrivate-1 && sym < yyPrivate+len(yyToknames) {
		return yyToknames[sym-yyPrivate+1]
	}
	return fmt.Sprintf("'%c'", sym)
}

func (r *lexResult) Errorf(format string) error {
	last := len(r.tokens)-1
	return &LexerError{Pos: r.TokenPos(last), Desc: fmt.Sprintf("%s %s", r.TokenSymString(last), format)}
}

func lex(data []byte, path string) (r *lexResult, err error) {
	r = newLexResult(path, len(data))
	err = lexData(data, r)
	return
}

func lexFile(path string) (r *lexResult, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	return lex(data, path)
}

type Backrefs [][2]int

func (stack *Backrefs) Push(i, fin int) {
	*stack = append(*stack, [2]int{i, fin})
}

func (stack *Backrefs) Pop() (i, fin int) {
	backref := (*stack)[len(*stack)-1]
	*stack = (*stack)[:len(*stack)-1]
	return backref[0], backref[1]
}
