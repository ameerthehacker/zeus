package compiler

import (
	"ameerthehacker/zeus/internal/common/error"
	"ameerthehacker/zeus/internal/lexer"
	"fmt"
)

type Compiler struct {
	lexer *lexer.Lexer
}

func NewCompiler(source string) *Compiler {
	return &Compiler{lexer: lexer.NewLexer(source)}
}

func (c *Compiler) Compile() []error.ZeusError {
	tokens, errors := c.lexer.Lex()

	for _, token := range tokens {
		fmt.Printf("%+v\n", token)
	}

	return errors
}
