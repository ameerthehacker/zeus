package zeus_compiler

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/codegen"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"tinygo.org/x/go-llvm"
)

type Compiler struct {
	codegen *codegen.Codegen
}

type EmitFileType string

const (
	EmitFileTypeLLVMIR EmitFileType = "ll"
	EmitFileTypeObject EmitFileType = "obj"
	EmitFileTypeASM    EmitFileType = "asm"
	EmitFileTypeEXE    EmitFileType = "exe"
)

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

func (c *Compiler) EmitFile(sourceFile *SourceFile, emitFileType EmitFileType, outputPath string) error {
	targetTriple := llvm.DefaultTargetTriple()
	target, err := llvm.GetTargetFromTriple(targetTriple)
	targetMachine := target.CreateTargetMachine(
		targetTriple,
		"generic",
		"",
		llvm.CodeGenLevelDefault,
		llvm.RelocDefault,
		llvm.CodeModelDefault,
	)


	if err != nil {
		return err
	}

	switch emitFileType {
	case EmitFileTypeLLVMIR:
		os.WriteFile(outputPath, []byte(sourceFile.Module.String()), 0644)
	case EmitFileTypeObject:
		fallthrough
	case EmitFileTypeASM:
		llvmCodegenType := llvm.AssemblyFile

		if emitFileType == EmitFileTypeObject {
			llvmCodegenType = llvm.ObjectFile
		}

		buffer, err := targetMachine.EmitToMemoryBuffer(sourceFile.Module.GetModule(), llvmCodegenType)
		if err != nil {
			return err
		}

		os.WriteFile(outputPath, buffer.Bytes(), 0644)
	default:
		// generate temp object file
		tempFile, err := os.CreateTemp(os.TempDir(), "zeus-*.o")

		if err != nil {
			return err
		}

		defer tempFile.Close()
		defer os.Remove(tempFile.Name())

		// write buffer to temp file
		buffer, err := targetMachine.EmitToMemoryBuffer(sourceFile.Module.GetModule(), llvm.ObjectFile)
		if err != nil {
			return err
		}
		tempFile.Write(buffer.Bytes())

		// link object file to platform executable
		var linkerCmd *exec.Cmd
		
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			linkerCmd = exec.Command("ld", tempFile.Name(), "-o", outputPath)
		} else {
			return fmt.Errorf("%s is not supported", runtime.GOOS)
		}

		err = linkerCmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}
