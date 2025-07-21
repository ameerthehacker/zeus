package zeus_compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/codegen"
	"github.com/ameerthehacker/zeus/internal/debug"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"tinygo.org/x/go-llvm"
)

const zeusRuntimeDir = "ZEUS_RUNTIME_DIR"

type Compiler struct {
	codegen *codegen.Codegen
	outputDir string
	targetMachine llvm.TargetMachine
	targetDataLayout llvm.TargetData
}

type EmitFileType string

const (
	// TODO: implement obj emit
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
	Exports []*zeus_value.Value
	IsEntryPoint bool
}

type SourceFileDependency struct {
	Span *token.Span
	SourceFile *SourceFile
}

type SourceFileErrorType int

const (
	SourceFileNotFound SourceFileErrorType = iota
	Unknown
)

type SourceFileError struct {
	Type SourceFileErrorType
	Message string
}

func NewSourceFileError(t SourceFileErrorType, message string) *SourceFileError {
	return &SourceFileError{
		Type:    t,
		Message: message,
	}
}

func (e *SourceFileError) Error() string {
	return e.Message
}

func NewCompiler(outputDir string) (*Compiler, error) {
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllAsmParsers()
	llvm.InitializeAllAsmPrinters()
	targetTriple := llvm.DefaultTargetTriple()
	target, err := llvm.GetTargetFromTriple(targetTriple)

	if err != nil {
		return nil, err
	}

	targetMachine := target.CreateTargetMachine(
		targetTriple,
		"generic",
		"",
		llvm.CodeGenLevelDefault,
		llvm.RelocDefault,
		llvm.CodeModelDefault,
	)
	targetDataLayout := targetMachine.CreateTargetData()
	return &Compiler{
		codegen: codegen.NewCodegen(),
		outputDir: outputDir,
		targetMachine: targetMachine,
		targetDataLayout: targetDataLayout,
	}, nil
}

func (c *Compiler) GetDependencies(program *ast.ProgramNode, sourcePath string) ([]*SourceFileDependency, []*zeus_error.ZeusError) {
	dependencies := []*SourceFileDependency{}
	errors := []*zeus_error.ZeusError{}

	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.ImportStmtNode:
			dependencyPath := module.ResolveFilePath(sourcePath, stmt.Source.Value)
			dependency, err := c.ReadSourceFile(dependencyPath)

			if err != nil && err.Type == SourceFileNotFound {
				errors = append(errors, zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "module not found", stmt.Source.Span))
				continue
			} else if err != nil {
				errors = append(errors, zeus_error.NewZeusError(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to read module %s", dependencyPath), stmt.Source.Span))
				continue
			}

			dependencies = append(dependencies, &SourceFileDependency{
				Span: stmt.Source.Span,
				SourceFile: dependency,
			})
		}
	}

	return dependencies, errors
}

func (c *Compiler) ReadSourceFile(path string) (*SourceFile, *SourceFileError) {
	content, err := os.ReadFile(path)

	if err != nil && os.IsNotExist(err) {
		return nil, NewSourceFileError(SourceFileNotFound, "file does not exist")
	} else if err != nil {
		return nil, NewSourceFileError(Unknown, err.Error())
	}

	return &SourceFile{
		Path: path,
		Source: string(content),
	}, nil
}

// RunOptimizationPasses runs LLVM optimization passes on the modules, specifically
// PlaceSafepoints and RewriteStatepointsForGC for garbage collection support
func (c *Compiler) RunOptimizationPasses(sourceFiles []*SourceFile) error {
	for _, sourceFile := range sourceFiles {
		if sourceFile.Module == nil {
			continue
		}

		// Get the LLVM module
		llvmModule := sourceFile.Module.GetModule()

		// Create PassBuilder options
		options := llvm.NewPassBuilderOptions()
		defer options.Dispose()

		// Enable debug logging for pass execution (optional)
		options.SetDebugLogging(debug.IsDebug())
		options.SetVerifyEach(false) // Verify after each pass for debugging

		// Run GC-related passes using the new PassBuilder system
		// PlaceSafepoints: Inserts safepoint polls at function entries and loop backedges
		// RewriteStatepointsForGC: Transforms calls to explicit statepoint relocations

		// Run the passes on the module
		err := llvmModule.RunPasses("mem2reg,place-safepoints", c.targetMachine, options)
		if err != nil {
			return fmt.Errorf("failed to run place-safepoints pass on module %s: %v", sourceFile.Path, err)
		}
		err = llvmModule.RunPasses("rewrite-statepoints-for-gc", c.targetMachine, options)
		if err != nil {
			return fmt.Errorf("failed to run rewrite-statepoints-for-gc pass on module %s: %v", sourceFile.Path, err)
		}
	}

	return nil
}

func (c *Compiler) Compile(entryFilePath string, emitFileType EmitFileType, outputPath string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Log(zeus_error.ErrorSeverityError, "an internal compiler error occured")
			fmt.Fprintln(os.Stderr, "Please create an issue on the github repo with the following information:")
			panic(r)
		}
	}()
	checkSourceFilesErrors := func(sourceFiles []*SourceFile) {
		hasErrors := false

		for _, sourceFile := range sourceFiles {
			if len(sourceFile.Errors) > 0 {
				hasErrors = true
				logger.PrettyPrintError(entryFilePath, sourceFile.Source, sourceFile.Errors)
			}
		}

		if hasErrors {
			os.Exit(1)
		}
	}
	entryPointSourceFile, err := c.ReadSourceFile(entryFilePath)
	entryPointSourceFile.IsEntryPoint = true

	if err != nil {
		logger.Log(zeus_error.ErrorSeverityError, err.Error())
		os.Exit(1)
	}

	// parse and collect dependencies
	sourceFiles := c.CollectDependencies(entryPointSourceFile)

	defer func() {
		if debug.IsDebug() {
			for _, sourceFile := range sourceFiles {
				if sourceFile.IRBuilder != nil {
					fmt.Printf("---:Zeus IR [%s]:---\n", sourceFile.Path)
					sourceFile.IRBuilder.Print()	
				}
			}
		}
	}()

	checkSourceFilesErrors(sourceFiles)
	// generate zeus IR
	sourceFiles = c.GenerateZeusIR(sourceFiles)
	checkSourceFilesErrors(sourceFiles)
	// type check the zeus IR
	sourceFiles = c.TypeCheck(sourceFiles)
	checkSourceFilesErrors(sourceFiles)
	// generate llvm IR
	sourceFiles = c.GenerateLLVMIR(sourceFiles)
	checkSourceFilesErrors(sourceFiles)

	optimizationError := c.RunOptimizationPasses(sourceFiles)
	if optimizationError != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to run optimization passes: %s", optimizationError.Error()))
		os.Exit(1)
	}

	// emit llvm object files
	objDir, emitError := c.EmitObjFiles(sourceFiles)

	if emitError != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to emit object files: %s", emitError.Error()))
		os.Exit(1)
	}

	linkError := LinkObjFiles(objDir, outputPath)
	if linkError != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to link object files: %s", linkError.Error()))
		os.Exit(1)
	}
}

func (c *Compiler) GenerateLLVMIR(sourceFiles []*SourceFile) []*SourceFile {
	for _, sourceFile := range sourceFiles {
		llvmModule := c.codegen.NewModule(sourceFile.Path, sourceFile.IsEntryPoint, c.targetDataLayout)
		zeus_error.Assert(sourceFile.IRBuilder != nil, "source file ir builder is nil")
		llvmModule.Generate(*sourceFile.IRBuilder)
		sourceFile.Module = llvmModule
	}

	return sourceFiles
}

func (c *Compiler) CheckMainFunction(sourceFile *SourceFile) {
	hasMainFunction := false

	sourceFile.IRBuilder.Walk(func(instr *ir.Instr) {
		if instr.Type == ir.InstrTypeDeclFunc {
			if ir.AsDeclFuncInstrInput(instr.Input).Function.Name == token.MAIN_FUNCTION_NAME {
				hasMainFunction = true
			}
		}
	}, func(block *ir.BasicBlock) {})

	if !hasMainFunction {
		sourceFile.Errors = append(sourceFile.Errors, zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "main function not found", nil))
	}
}

func (c *Compiler) TypeCheck(sourceFiles []*SourceFile) []*SourceFile {
	for _, sourceFile := range sourceFiles {
		zeus_error.Assert(sourceFile.IRBuilder != nil, "source file ir builder is nil")
		typeChecker := ir.NewTypeChecker(sourceFile.IRBuilder)
		errors := typeChecker.TypeCheck()
		sourceFile.Errors = append(sourceFile.Errors, errors...)
	}

	return sourceFiles
}

func (c *Compiler) GenerateZeusIR(sourceFiles []*SourceFile) []*SourceFile {
	irModuleFilePathMap := map[string]*ir.IRModule{}

	for _, sourceFile := range sourceFiles {
		// if the source file is already in the map, skip it
		_, ok := irModuleFilePathMap[sourceFile.Path]
		if ok {
			continue
		}
		// generate zeus IR and cache it
		zeus_error.Assert(sourceFile.Program != nil, "source file program is nil")
		irBuilder := ir.NewIRBuilder()
		sourceFile.IRBuilder = irBuilder
		irModule := ir.NewIRModule(irBuilder, sourceFile.Path, func(modulePath string) *ir.IRModule {
			irModule, ok := irModuleFilePathMap[modulePath]
			zeus_error.Assert(ok, fmt.Sprintf("IR module not found %s", modulePath))
			return irModule
		})
		irModuleFilePathMap[sourceFile.Path] = irModule
		errors := irModule.Generate(sourceFile.Program)
		sourceFile.Errors = append(sourceFile.Errors, errors...)
		if sourceFile.IsEntryPoint {
			c.CheckMainFunction(sourceFile)
		}
	}

	return sourceFiles
}

func (c *Compiler) CollectDependencies(entry *SourceFile) []*SourceFile {
	sourceFiles := []*SourceFile{}
	queue := []*SourceFileDependency{{
		Span: nil,
		SourceFile: entry,
	}}
	visited := map[string]bool{}
	inProgress := map[string]bool{}

	// BFS traversal so that we can generate the source files in the order they are imported
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current.SourceFile.Path] {
			continue
		}

		inProgress[current.SourceFile.Path] = true
		sourceFile := c.CompileFile(current.SourceFile)
		sourceFiles = append([]*SourceFile{sourceFile}, sourceFiles...)

		if sourceFile.Program != nil {
			dependencies, errors := c.GetDependencies(sourceFile.Program, sourceFile.Path)

			for _, dependency := range dependencies {
				if inProgress[dependency.SourceFile.Path] {
					sourceFile.Errors = append(sourceFile.Errors, zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "circular dependency detected", dependency.Span))
					continue
				}
			}

			// append the module resolution errors
			sourceFile.Errors = append(sourceFile.Errors, errors...)
			queue = append(queue, dependencies...)
		}

		visited[current.SourceFile.Path] = true
	}

	return sourceFiles
}

func (c *Compiler) CompileFile(input *SourceFile) *SourceFile {
	lexer := lexer.NewLexer(input.Source)
	tokens, lexerErrors := lexer.Lex()

	if len(lexerErrors) > 0 {
		input.Errors = lexerErrors
		return input
	}

	parser := parser.NewParser(tokens)
	program, parserErrors := parser.ParseProgram()

	if len(parserErrors) > 0 {
		input.Errors = parserErrors
		return input
	}

	input.Program = program
	return input
}

func (c *Compiler) EmitObjFiles(sourceFiles []*SourceFile) (string, error) {
	objDir, err := os.MkdirTemp(c.outputDir, "zeus-obj-*")
	llDir, llDirErr := os.MkdirTemp(c.outputDir, "zeus-ll-*")
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
		buffer, err := c.targetMachine.EmitToMemoryBuffer(sourceFile.Module.GetModule(), llvm.ObjectFile)
		if err != nil {
			return "", err
		}
		_, err = tempFile.Write(buffer.Bytes())
		if err != nil {
			return "", err
		}
		// write llvm ir to temp file
		if debug.IsDebug() && llDirErr == nil {
			tempLLFile, err := os.CreateTemp(llDir, "zeus-*.ll")
			if err != nil {
				continue
			}
			tempLLFile.Write([]byte(sourceFile.Module.String()))
			defer tempLLFile.Close()
		}
	}

	return objDir, nil
}

func GetRuntimeDir() string {
	runtimeDir := os.Getenv(zeusRuntimeDir)
	if runtimeDir == "" {
		runtimeDir, err := os.Getwd()
		if err != nil {
			logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to get zeus home path: %s", err.Error()))
			os.Exit(1)
		}
		return filepath.Join(runtimeDir, "runtime", "out")
	}
	return runtimeDir
}

func LinkObjFiles(objDir string, outputPath string) error {
	objFiles, err := filepath.Glob(fmt.Sprintf("%s/zeus-*.o", objDir))
	if err != nil {
		return err
	}
	runtimeDir := GetRuntimeDir()
	runtimeObjFiles, err := filepath.Glob(fmt.Sprintf("%s/*.o", runtimeDir))
	if err != nil {
		return err
	}
	objFiles = append(objFiles, runtimeObjFiles...)
	// link object file to platform executable
	var linkerCmd *exec.Cmd
		
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		linker := "gcc"
		if runtime.GOOS == "darwin" {
			linker = "clang"
		}
		args := []string{"-fno-omit-frame-pointer"}
		args = append(args, objFiles...)
		args = append(args, "-o", outputPath)
		linkerCmd = exec.Command(linker, args...)
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