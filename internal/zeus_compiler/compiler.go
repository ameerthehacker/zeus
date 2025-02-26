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

func (c *Compiler) Compile(entryFilePath string, emitFileType EmitFileType, outputPath string) {
	entryPointInput, err := ReadSourceFile(entryFilePath)
	
	if err != nil {
		logger.Log(zeus_error.ErrorSeverityError, err.Error())
		os.Exit(1)
	}

	sourceFiles := c.GenerateSourceFiles(*entryPointInput)
	hasCompilationErrors := false

	for _, sourceFile := range sourceFiles {
		if len(sourceFile.Errors) > 0 {
			logger.PrettyPrintError(sourceFile.Path, sourceFile.Source, sourceFile.Errors)
			hasCompilationErrors = true
		}
	}

	if hasCompilationErrors {
		os.Exit(1)
	}
}

func (c *Compiler) GenerateSourceFiles(entry Input) []*SourceFile {
	defer func() {
		if r := recover(); r != nil {
			logger.Log(zeus_error.ErrorSeverityError, "an internal compiler error occured")
			fmt.Fprintln(os.Stderr, "Please create an issue on the github repo with the following information:")
			panic(r)
		}
	}()
	sourceFiles := []*SourceFile{}
	queue := []Input{entry}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		sourceFile := c.CompileFile(current)
		sourceFiles = append(sourceFiles, sourceFile)

		dependencies, errors := GetDependencies(sourceFile.Program, sourceFile.Path)
		// append the module resolution errors
		sourceFile.Errors = append(sourceFile.Errors, errors...)

		for _, dependency := range dependencies {
			queue = append(queue, *dependency)
		}
	}

	return sourceFiles
}

func (c *Compiler) CompileFile(input Input) *SourceFile {
	lexer := lexer.NewLexer(input.Source)
	tokens, lexerErrors := lexer.Lex()

	if len(lexerErrors) > 0 {
		return &SourceFile{
			Path: input.Path,
			Source: input.Source,
			Errors: lexerErrors,
		}
	}

	parser := parser.NewParser(tokens)
	program, parserErrors := parser.ParseProgram()

	if len(parserErrors) > 0 {
		return &SourceFile{
			Path: input.Path,
			Source: input.Source,
			Errors: parserErrors,
		}
	}

	return &SourceFile{
		Path: input.Path,
		Source: input.Source,
		Program: program,
		Errors: []*zeus_error.ZeusError{},
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
