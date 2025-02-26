package zeus_compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	EmitFileTypeObject EmitFileType = "obj"
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
	checkSourceFilesErrors := func(sourceFiles []*SourceFile) {
		hasErrors := false

		for _, sourceFile := range sourceFiles {
			if len(sourceFile.Errors) > 0 {
				logger.PrettyPrintError(sourceFile.Path, sourceFile.Source, sourceFile.Errors)
				hasErrors = true
			}
		}

		if hasErrors {
			os.Exit(1)
		}
	}
	entryPointInput, err := ReadSourceFile(entryFilePath)
	
	if err != nil {
		logger.Log(zeus_error.ErrorSeverityError, err.Error())
		os.Exit(1)
	}

	// parse and generate AST
	sourceFiles := c.GenerateSourceFiles(*entryPointInput)
	checkSourceFilesErrors(sourceFiles)
	// generate zeus IR
	sourceFiles = c.GenerateZeusIR(sourceFiles)
	checkSourceFilesErrors(sourceFiles)
	// type check the zeus IR
	sourceFiles = c.TypeCheckSourceFiles(sourceFiles)
	checkSourceFilesErrors(sourceFiles)
	// generate llvm IR
	sourceFiles = c.GenerateLLVMIR(sourceFiles)
	checkSourceFilesErrors(sourceFiles)
	// emit llvm object files
	objDir, emitError := c.EmitFiles(sourceFiles)

	if emitError != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to emit object files: %s", emitError.Error()))
		os.Exit(1)
	}

	if emitFileType == EmitFileTypeEXE {
		linkError := LinkObjFiles(objDir, outputPath)
		if linkError != nil {
			logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to link object files: %s", linkError.Error()))
			os.Exit(1)
		}
	}
}

func (c *Compiler) GenerateLLVMIR(sourceFiles []*SourceFile) []*SourceFile {
	for _, sourceFile := range sourceFiles {
		llvmModule := c.codegen.NewModule(sourceFile.Path)
		zeus_error.Assert(sourceFile.IRBuilder != nil, "source file ir builder is nil")
		llvmModule.Generate(*sourceFile.IRBuilder)
		sourceFile.Module = llvmModule
	}

	return sourceFiles
}

func (c *Compiler) TypeCheckSourceFiles(sourceFiles []*SourceFile) []*SourceFile {
	for _, sourceFile := range sourceFiles {
		zeus_error.Assert(sourceFile.IRBuilder != nil, "source file ir builder is nil")
		typeChecker := ir.NewTypeChecker(sourceFile.IRBuilder)
		errors := typeChecker.TypeCheck()
		sourceFile.Errors = append(sourceFile.Errors, errors...)
	}

	return sourceFiles
}

func (c *Compiler) GenerateZeusIR(sourceFiles []*SourceFile) []*SourceFile {
	sourcesFilesPathMap := map[string]*SourceFile{}

	for _, sourceFile := range sourceFiles {
		sourcesFilesPathMap[sourceFile.Path] = sourceFile
	}

	for _, sourceFile := range sourceFiles {
		irBuilder := ir.NewIRBuilder()
		sourceFile.IRBuilder = irBuilder
		irModule := ir.NewIRModule(irBuilder)
		zeus_error.Assert(sourceFile.Program != nil, "source file program is nil")
		errors := irModule.Generate(sourceFile.Program)
		sourceFile.Errors = append(sourceFile.Errors, errors...)
	}

	return sourceFiles
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

func (c *Compiler) EmitFiles(sourceFiles []*SourceFile) (string, error) {
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
		return "", err
	}

	objDir, err := os.MkdirTemp(os.TempDir(), "zeus-obj-*")
	if err != nil {
		return "", err
	}

	for _, sourceFile := range sourceFiles {
		// generate temp object file
		tempFile, err := os.CreateTemp(objDir, "zeus-*.o")

		if err != nil {
			return "", err
		}

		defer tempFile.Close()

		// write buffer to temp file
		zeus_error.Assert(sourceFile.Module != nil, "source file module is nil")
		buffer, err := targetMachine.EmitToMemoryBuffer(sourceFile.Module.GetModule(), llvm.ObjectFile)
		if err != nil {
			return "", err
		}
		tempFile.Write(buffer.Bytes())
	}

	return objDir, nil
}

func LinkObjFiles(objDir string, outputPath string) error {
	objFiles, err := filepath.Glob(fmt.Sprintf("%s/zeus-*.o", objDir))
	if err != nil {
		return err
	}
	// link object file to platform executable
	var linkerCmd *exec.Cmd
		
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		args := []string{}
		args = append(args, objFiles...)
		args = append(args, "-o", outputPath)
		linkerCmd = exec.Command("ld", args...)
		linkerCmd.Stdout = os.Stdout
		linkerCmd.Stderr = os.Stderr
	} else {
		return fmt.Errorf("%s is not supported", runtime.GOOS)
	}

	err = linkerCmd.Run()
	if err != nil {
		return err
	}

	return nil
}