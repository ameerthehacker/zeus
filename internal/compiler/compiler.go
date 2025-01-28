package compiler

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/error"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
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
	expr, parserErrors := parser.ParseExpr()

	if len(parserErrors) > 0 {
		return parserErrors
	}

	fmt.Printf("%s\n", expr.PrettyString())
	fmt.Printf("%s\n", expr)
	
	return []*error.ZeusError{}
}
