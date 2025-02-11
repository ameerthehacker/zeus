package compiler

import (
	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type Compiler struct {}

type SourceFile struct {
	Path string
	Source string
	Program *ast.ProgramNode
	Errors []*zeus_error.ZeusError
}

type Input struct {
	Path string
	Source string
}

func NewCompiler() *Compiler {
	return &Compiler{}
}

func (c *Compiler) CompileFile(entryPoint Input) *SourceFile {
	lexer := lexer.NewLexer(entryPoint.Source)
	tokens, lexerErrors := lexer.Lex()

	if len(lexerErrors) > 0 {
		return &SourceFile{
			Path: entryPoint.Path,
			Source: entryPoint.Source,
			Errors: lexerErrors,
		}
	}

	parser := parser.NewParser(tokens)
	program, parserErrors := parser.ParseProgram()

	if len(parserErrors) > 0 {
		return &SourceFile{
			Path: entryPoint.Path,
			Source: entryPoint.Source,
			Errors: parserErrors,
		}
	}

	irBuilder := ir.NewIRBuilder()
	irGen := ir.NewIRGen(irBuilder)
	irErrors := irGen.Generate(program)

	if len(irErrors) > 0 {
		return &SourceFile{
			Path: entryPoint.Path,
			Source: entryPoint.Source,
			Errors: irErrors,
		}
	}

	irBuilder.Print()

	return &SourceFile{
		Path: entryPoint.Path,
		Source: entryPoint.Source,
		Program: program,
		Errors: []*zeus_error.ZeusError{},
	}
}
