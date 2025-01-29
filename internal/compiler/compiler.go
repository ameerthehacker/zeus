package compiler

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/error"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
)

type Compiler struct {}

func NewCompiler() *Compiler {
	return &Compiler{}
}

func (c *Compiler) Compile(source string) []*error.ZeusError {
	lexer := lexer.NewLexer(source)
	tokens, lexerErrors := lexer.Lex()

	if len(lexerErrors) > 0 {
		return lexerErrors
	}

	parser := parser.NewParser(tokens)
	program, parserErrors := parser.ParseProgram()

	if len(parserErrors) > 0 {
		return parserErrors
	}

	fmt.Printf("%s\n", program.PrettyString())
	// fmt.Printf("%s\n", program)
	
	return []*error.ZeusError{}
}
