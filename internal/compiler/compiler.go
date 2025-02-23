package compiler

import (
	"fmt"
	"os"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/codegen"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type Compiler struct {
	codegen *codegen.Codegen
}

type SourceFile struct {
	Path string
	Source string
	Program *ast.ProgramNode
	Module *codegen.CodegenModule
	Errors []*zeus_error.ZeusError
	IRBuilder *ir.IRBuilder
}

type Input struct {
	Path string
	Source string
}

func NewCompiler() *Compiler {
	return &Compiler{
		codegen: codegen.NewCodegen(),
	}
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

	defer func() {
		if r := recover(); r != nil {
			logger.Log(zeus_error.ErrorSeverityError, "an internal compiler error")
			fmt.Fprintln(os.Stderr, "Please create an issue on the github repo with the following information:")
			fmt.Fprintln(os.Stderr, "---:GENERATED ZEUS IR:---")
			fmt.Fprintln(os.Stderr, irBuilder.String())
			panic(r)
		}
	}()

	tc := ir.NewTypeChecker(irBuilder)
	tcErrors := tc.TypeCheck()

	if len(tcErrors) > 0 {
		return &SourceFile{
			Path: entryPoint.Path,
			Source: entryPoint.Source,
			Errors: tcErrors,
		}
	}

	codegenModule := c.codegen.NewModule(entryPoint.Path)
	codegenModule.Generate(*irBuilder)

	return &SourceFile{
		Path: entryPoint.Path,
		Source: entryPoint.Source,
		Program: program,
		Errors: []*zeus_error.ZeusError{},
		Module: codegenModule,
		IRBuilder: irBuilder,
	}
}
