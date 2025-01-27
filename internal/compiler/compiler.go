package compiler

import (
	"ameerthehacker/zeus/internal/error"
	"ameerthehacker/zeus/internal/lexer"
	"ameerthehacker/zeus/internal/parser"
	"fmt"
)

type Compiler struct {
	lexer *lexer.Lexer
}

func NewCompiler(source string) *Compiler {
	return &Compiler{lexer: lexer.NewLexer(source)}
}

func (c *Compiler) Compile() []*error.ZeusError {
	tokens, lexerErrors := c.lexer.Lex()

	if len(lexerErrors) > 0 {
		return lexerErrors
	}

	parser := parser.NewParser(tokens)
	expr, parserErrors := parser.ParseExpr(0)

	if len(parserErrors) > 0 {
		return parserErrors
	}

	fmt.Printf("%s\n", expr)

	return []*error.ZeusError{}
}
