package zeus_compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/codegen"
	"github.com/ameerthehacker/zeus/internal/debug"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/util"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"tinygo.org/x/go-llvm"
)

type BuildMode string

const (
	BuildModeDebug   BuildMode = "debug"
	BuildModeRelease BuildMode = "release"
)

type Compiler struct {
	codegen          *codegen.Codegen
	targetMachine    llvm.TargetMachine
	targetDataLayout llvm.TargetData
	mode             BuildMode
	emitIR           bool
	irDir            string
	cwd              string
}

type SourceFile struct {
	Path         string
	Source       string
	Program      *ast.ProgramNode
	Module       *codegen.CodegenModule
	Errors       []*zeus_error.ZeusError
	IRBuilder    *ir.IRBuilder
	Exports      []*zeus_value.Value
	IsEntryPoint bool
	HIRText      string // captured after GenerateZeusIR, before TypeCheck and LowerIR
	LIRText      string // captured after LowerIR, before GenerateLLVMIR
}

type SourceFileDependency struct {
	Span       *token.Span
	SourceFile *SourceFile
}

type SourceFileErrorType int

const (
	SourceFileNotFound SourceFileErrorType = iota
	Unknown
)

type SourceFileError struct {
	Type    SourceFileErrorType
	Message string
}

// if any internal compiler exception happens we need to let the user know
func TrapCompilerException() {
	if r := recover(); r != nil {
		logger.Log(zeus_error.ErrorSeverityError, "an internal compiler error occured")
		fmt.Fprintln(os.Stderr, "Please create an issue on the github repo with the following information:")
		panic(r)
	}
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

func NewCompiler(mode BuildMode) (*Compiler, error) {
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllAsmParsers()
	llvm.InitializeAllAsmPrinters()
	targetTriple := llvm.DefaultTargetTriple()

	// Set minimum macOS deployment target to match runtime build
	if runtime.GOOS == "darwin" {
		// Parse the triple and update the OS version
		// Format: arch-vendor-os-version
		parts := strings.Split(targetTriple, "-")
		macosVersion, err := util.GetMacOSVersion()

		if err != nil {
			return nil, err
		}

		if len(parts) >= 3 {
			// Replace or add the version to match our deployment target
			targetTriple = parts[0] + "-" + parts[1] + "-macosx" + macosVersion
		}
	}

	target, err := llvm.GetTargetFromTriple(targetTriple)

	if err != nil {
		return nil, err
	}

	codeGenLevel := llvm.CodeGenLevelDefault
	if mode == BuildModeRelease {
		codeGenLevel = llvm.CodeGenLevelAggressive
	}

	targetMachine := target.CreateTargetMachine(
		targetTriple,
		"generic",
		"",
		codeGenLevel,
		llvm.RelocDefault,
		llvm.CodeModelDefault,
	)
	targetDataLayout := targetMachine.CreateTargetData()
	return &Compiler{
		codegen:          codegen.NewCodegen(),
		targetMachine:    targetMachine,
		targetDataLayout: targetDataLayout,
		mode:             mode,
	}, nil
}

func (c *Compiler) EnableIREmission(irDir string, cwd string) {
	c.emitIR = true
	c.irDir = irDir
	c.cwd = cwd
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
				Span:       stmt.Source.Span,
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
		Path:   path,
		Source: string(content),
	}, nil
}

// isPrimordialFactoryFunction reports whether fn is a primordial factory function.
// Primordial factory functions are always named "zeus_new_<ClassName>" and are the
// only functions that use LinkOnceODRLinkage (each compilation unit emits an identical
// copy and the linker keeps exactly one). Detecting by name is more explicit than
// relying solely on linkage type.
func isPrimordialFactoryFunction(fn llvm.Value) bool {
	return strings.HasPrefix(fn.Name(), codegen.FactoryFunctionPrefix) &&
		fn.Linkage() == llvm.LinkOnceODRLinkage &&
		!fn.IsDeclaration()
}

// pinPrimordialFactoryFunctions keeps primordial factory functions alive through
// llvm.LinkModules, which silently drops LinkOnceODR functions that have no
// intra-module callers (the Zig runtime calls them, so they look unused in LLVM IR).
//
// pinned tracks names already promoted in a prior module. The first module that
// defines a factory function is promoted to ExternalLinkage (strong symbol);
// subsequent modules keep LinkOnceODR and LinkModules resolves them against the
// External definition already in the merged module without a "multiply defined" error.
func pinPrimordialFactoryFunctions(sourceMod llvm.Module, pinned map[string]bool) {
	var fns []llvm.Value
	for fn := sourceMod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		if !isPrimordialFactoryFunction(fn) {
			continue
		}
		if !pinned[fn.Name()] {
			fn.SetLinkage(llvm.ExternalLinkage)
			pinned[fn.Name()] = true
		}
		fns = append(fns, fn)
	}
	if len(fns) == 0 {
		return
	}
	ptrType := llvm.PointerType(sourceMod.Context().VoidType(), 0)
	vals := make([]llvm.Value, len(fns))
	for i, fn := range fns {
		vals[i] = fn
	}
	usedArray := llvm.ConstArray(ptrType, vals)
	usedGlobal := llvm.AddGlobal(sourceMod, usedArray.Type(), "llvm.used")
	usedGlobal.SetInitializer(usedArray)
	usedGlobal.SetLinkage(llvm.AppendingLinkage)
	usedGlobal.SetSection("llvm.metadata")
}

func (c *Compiler) runReleasePasses(merged llvm.Module) error {
	options := llvm.NewPassBuilderOptions()
	defer options.Dispose()

	options.SetDebugLogging(debug.IsDebug())
	options.SetVerifyEach(false)

	if err := merged.RunPasses("default<O3>", c.targetMachine, options); err != nil {
		return fmt.Errorf("failed to run release optimization passes: %v", err)
	}

	return nil
}

func (c *Compiler) mergeModules(sourceFiles []*SourceFile) (llvm.Module, error) {
	merged := c.codegen.NewMergeTarget(c.targetDataLayout.String(), c.targetMachine.Triple())
	pinned := make(map[string]bool)

	for _, sourceFile := range sourceFiles {
		if sourceFile.Module == nil {
			continue
		}
		pinPrimordialFactoryFunctions(sourceFile.Module.GetModule(), pinned)
		if err := llvm.LinkModules(merged, sourceFile.Module.GetModule()); err != nil {
			return llvm.Module{}, fmt.Errorf("failed to link module %s: %v", sourceFile.Path, err)
		}
	}

	return merged, nil
}

func (c *Compiler) Check(entryFilePath string) []*SourceFile {
	defer TrapCompilerException()

	checkSourceFilesWarnings := func(sourceFiles []*SourceFile) {
		for _, sourceFile := range sourceFiles {
			if len(sourceFile.Errors) > 0 {
				warningSeverityErrors := []*zeus_error.ZeusError{}
				lastWarningLine := -1
				for _, err := range sourceFile.Errors {
					if err.Severity != zeus_error.ErrorSeverityWarning {
						continue
					}
					if lastWarningLine == err.Span.Start.Line {
						continue
					}
					warningSeverityErrors = append(warningSeverityErrors, err)
					lastWarningLine = err.Span.Start.Line
				}

				if len(warningSeverityErrors) > 0 {
					logger.PrettyPrintError(entryFilePath, sourceFile.Source, warningSeverityErrors)
				}
			}
		}
	}

	checkSourceFilesErrors := func(sourceFiles []*SourceFile) {
		hasErrors := false

		for _, sourceFile := range sourceFiles {
			if len(sourceFile.Errors) > 0 {
				// Filter errors to only include severity Error
				errorSeverityErrors := []*zeus_error.ZeusError{}
				lastErrorLine := -1
				for _, err := range sourceFile.Errors {
					if err.Severity != zeus_error.ErrorSeverityError {
						continue
					}
					if lastErrorLine == err.Span.Start.Line {
						continue
					}
					errorSeverityErrors = append(errorSeverityErrors, err)
					lastErrorLine = err.Span.Start.Line
				}

				if len(errorSeverityErrors) > 0 {
					hasErrors = true
					logger.PrettyPrintError(entryFilePath, sourceFile.Source, errorSeverityErrors)
				}
			}
		}

		if hasErrors {
			// before exiting print warnings
			checkSourceFilesWarnings(sourceFiles)
			os.Exit(1)
		}
	}

	entryPointSourceFile, err := c.ReadSourceFile(entryFilePath)

	if err != nil {
		logger.Log(zeus_error.ErrorSeverityError, err.Error())
		os.Exit(1)
	}

	entryPointSourceFile.IsEntryPoint = true
	// parse and collect dependencies
	sourceFiles := c.CollectDependencies(entryPointSourceFile)

	checkSourceFilesErrors(sourceFiles)
	// generate zeus IR
	sourceFiles = c.GenerateZeusIR(sourceFiles)
	checkSourceFilesErrors(sourceFiles)
	// snapshot HIR from the IR generator before TypeCheck modifies anything
	if c.emitIR {
		for _, sf := range sourceFiles {
			if sf.IRBuilder != nil {
				sf.HIRText = formatZeusIR(sf.IRBuilder, "HIR", sf.Path)
			}
		}
		c.writeHIRFiles(sourceFiles)
	}
	// type check the zeus IR
	sourceFiles = c.TypeCheck(sourceFiles)
	checkSourceFilesErrors(sourceFiles)
	// lower high-level IR constructs (e.g., string<->u8[] casts)
	sourceFiles = c.LowerIR(sourceFiles)
	// snapshot LIR after lowering
	if c.emitIR {
		for _, sf := range sourceFiles {
			if sf.IRBuilder != nil {
				sf.LIRText = formatZeusIR(sf.IRBuilder, "LIR", sf.Path)
			}
		}
	}
	// generate llvm IR
	sourceFiles = c.GenerateLLVMIR(sourceFiles)
	checkSourceFilesErrors(sourceFiles)
	checkSourceFilesWarnings(sourceFiles)
	if c.emitIR {
		if err := c.emitIRFiles(sourceFiles); err != nil {
			logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to emit IR files: %s", err.Error()))
			os.Exit(1)
		}
	}

	return sourceFiles
}

func (c *Compiler) Compile(entryFilePath string, outDir string, outputPath string) {
	sourceFiles := c.Check(entryFilePath)
	if c.mode == BuildModeRelease {
		c.compileRelease(sourceFiles, outDir, outputPath)
	} else {
		c.compileDebug(sourceFiles, outDir, outputPath)
	}
}

func (c *Compiler) compileDebug(sourceFiles []*SourceFile, outDir string, outputPath string) {
	objFiles, emitError := c.EmitObjFiles(sourceFiles, outDir)
	if emitError != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to emit object files: %s", emitError.Error()))
		os.Exit(1)
	}

	if linkError := linkObjFiles(objFiles, outputPath); linkError != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to link object files: %s", linkError.Error()))
		os.Exit(1)
	}
}

func (c *Compiler) compileRelease(sourceFiles []*SourceFile, outputDir string, outputPath string) {
	merged, mergeErr := c.mergeModules(sourceFiles)
	if mergeErr != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to merge modules: %s", mergeErr.Error()))
		os.Exit(1)
	}

	if passErr := c.runReleasePasses(merged); passErr != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to run optimization passes: %s", passErr.Error()))
		os.Exit(1)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to create output dir: %s", err.Error()))
		os.Exit(1)
	}

	buffer, emitErr := c.targetMachine.EmitToMemoryBuffer(merged, llvm.ObjectFile)
	if emitErr != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to emit object file: %s", emitErr.Error()))
		os.Exit(1)
	}

	objPath := filepath.Join(outputDir, "program.o")
	if err := os.WriteFile(objPath, buffer.Bytes(), 0644); err != nil {
		logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to write object file: %s", err.Error()))
		os.Exit(1)
	}

	if linkError := linkObjFiles([]string{objPath}, outputPath); linkError != nil {
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

func (c *Compiler) TypeCheck(sourceFiles []*SourceFile) []*SourceFile {
	for _, sourceFile := range sourceFiles {
		zeus_error.Assert(sourceFile.IRBuilder != nil, "source file ir builder is nil")
		typeChecker := ir.NewTypeChecker(sourceFile.IRBuilder, sourceFile.IsEntryPoint)
		errors := typeChecker.TypeCheck()
		sourceFile.Errors = append(sourceFile.Errors, errors...)
	}

	return sourceFiles
}

func (c *Compiler) LowerIR(sourceFiles []*SourceFile) []*SourceFile {
	for _, sourceFile := range sourceFiles {
		zeus_error.Assert(sourceFile.IRBuilder != nil, "source file ir builder is nil")
		lowerer := ir.NewLowerer(sourceFile.IRBuilder)
		lowerer.Lower()
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
		irModule := ir.NewIRModule(irBuilder, sourceFile.Path, sourceFile.IsEntryPoint, func(modulePath string) *ir.IRModule {
			irModule, ok := irModuleFilePathMap[modulePath]
			zeus_error.Assert(ok, fmt.Sprintf("IR module not found %s", modulePath))
			return irModule
		})
		irModuleFilePathMap[sourceFile.Path] = irModule
		errors := irModule.Generate(sourceFile.Program)
		sourceFile.Errors = append(sourceFile.Errors, errors...)
	}

	return sourceFiles
}

func (c *Compiler) CollectDependencies(entry *SourceFile) []*SourceFile {
	sourceFiles := []*SourceFile{}
	queue := []*SourceFileDependency{{
		Span:       nil,
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

func sourceFileHash(sourceFile *SourceFile) string {
	h := sha256.New()
	h.Write([]byte(sourceFile.Path))
	h.Write([]byte("\n"))
	h.Write([]byte(sourceFile.Source))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Compiler) EmitObjFiles(sourceFiles []*SourceFile, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	objFiles := []string{}

	for _, sourceFile := range sourceFiles {
		hash := sourceFileHash(sourceFile)
		objPath := filepath.Join(outputDir, hash+".o")

		if _, err := os.Stat(objPath); err == nil {
			// cache hit — reuse existing object file
			objFiles = append(objFiles, objPath)
			continue
		}

		// cache miss — compile and write object file
		zeus_error.Assert(sourceFile.Module != nil, "source file module is nil")
		buffer, err := c.targetMachine.EmitToMemoryBuffer(sourceFile.Module.GetModule(), llvm.ObjectFile)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(objPath, buffer.Bytes(), 0644); err != nil {
			return nil, err
		}
		objFiles = append(objFiles, objPath)
	}

	return objFiles, nil
}

func formatZeusIR(b *ir.IRBuilder, phase string, filePath string) string {
	var sb strings.Builder
	sb.WriteString("; Zeus " + phase + "\n")
	sb.WriteString("; source: " + filePath + "\n\n")
	sb.WriteString(b.String())
	return sb.String()
}

// writeHIRFiles writes only .zhir files so HIR is available even when type errors abort compilation.
func (c *Compiler) writeHIRFiles(sourceFiles []*SourceFile) {
	for _, sf := range sourceFiles {
		relPath, err := filepath.Rel(c.cwd, sf.Path)
		if err != nil {
			relPath = filepath.Base(sf.Path)
		}
		relBase := strings.TrimSuffix(relPath, filepath.Ext(relPath))
		outBase := filepath.Join(c.irDir, relBase)
		if err := os.MkdirAll(filepath.Dir(outBase), 0755); err != nil {
			logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to create IR directory: %s", err.Error()))
			return
		}
		if err := os.WriteFile(outBase+".zhir", []byte(sf.HIRText), 0644); err != nil {
			logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to write HIR file: %s", err.Error()))
		}
	}
}

func (c *Compiler) emitIRFiles(sourceFiles []*SourceFile) error {
	for _, sf := range sourceFiles {
		relPath, err := filepath.Rel(c.cwd, sf.Path)
		if err != nil {
			relPath = filepath.Base(sf.Path)
		}
		relBase := strings.TrimSuffix(relPath, filepath.Ext(relPath))
		outBase := filepath.Join(c.irDir, relBase)

		if err := os.MkdirAll(filepath.Dir(outBase), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(outBase+".zhir", []byte(sf.HIRText), 0644); err != nil {
			return err
		}
		if err := os.WriteFile(outBase+".zlir", []byte(sf.LIRText), 0644); err != nil {
			return err
		}
		if sf.Module != nil {
			if err := os.WriteFile(outBase+".ll", []byte(sf.Module.GetModule().String()), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func getZeusHomeDir() string {
	zeusHome := os.Getenv(module.ZeusHomeEnvVar)
	if zeusHome == "" {
		// Resolve symlinks to get the actual binary location
		execPath, err := os.Executable()
		if err != nil {
			logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to get zeus executable path: %s", err.Error()))
			os.Exit(1)
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			logger.Log(zeus_error.ErrorSeverityError, fmt.Sprintf("failed to resolve symlinks: %s", err.Error()))
			os.Exit(1)
		}
		binDir := filepath.Dir(execPath)
		// Check if we're in a bin/ directory structure (e.g., Homebrew installation)
		if filepath.Base(binDir) == "bin" {
			zeusHome = filepath.Dir(binDir)
		} else {
			zeusHome = binDir
		}
	}

	return zeusHome
}

func getRuntimeDir() string {
	zeusHome := getZeusHomeDir()
	// Runtime objects always live at <zeus_home>/runtime/zig-out/out, whether ZEUS_HOME
	// is derived from the executable path or set explicitly via the env var.
	return filepath.Join(zeusHome, "runtime", "zig-out", "out")
}

func getBDWGCLibDir() string {
	zeusHome := getZeusHomeDir()

	return filepath.Join(zeusHome, "third_party", "bdwgc", "lib")
}

func linkObjFiles(objFiles []string, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	runtimeDir := getRuntimeDir()
	runtimeObjFiles, err := filepath.Glob(fmt.Sprintf("%s/*.o", runtimeDir))
	if err != nil {
		return err
	}
	objFiles = append(objFiles, runtimeObjFiles...)
	// link object file to platform executable
	var linkerCmd *exec.Cmd
	linkerArgs := []string{}

	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		linker := "gcc"
		if runtime.GOOS == "darwin" {
			macosVersion, err := util.GetMacOSVersion()

			if err != nil {
				return err
			}

			linker = "clang"
			// Set minimum macOS deployment target to match runtime build
			linkerArgs = append(linkerArgs, "-mmacosx-version-min="+macosVersion)
			// Get the correct SDK path
			sdkPathCmd := exec.Command("xcrun", "--show-sdk-path")
			sdkPathOutput, err := sdkPathCmd.Output()
			if err == nil {
				sdkPath := strings.TrimSpace(string(sdkPathOutput))
				linkerArgs = append(linkerArgs, "-isysroot", sdkPath)
			} else {
				// Fallback: try to find SDK manually
				fallbackSDKPath := "/Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX.sdk"
				if _, err := os.Stat(fallbackSDKPath); err == nil {
					linkerArgs = append(linkerArgs, "-isysroot", fallbackSDKPath)
				} else {
					// If both xcrun and fallback fail, try CommandLineTools path
					cmdToolsSDKPath := "/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk"
					if _, err := os.Stat(cmdToolsSDKPath); err == nil {
						linkerArgs = append(linkerArgs, "-isysroot", cmdToolsSDKPath)
					}
					// If all fail, continue without explicit sysroot (may still work with system defaults)
				}
			}
		}
		linkerArgs = append(linkerArgs, objFiles...)
		linkerArgs = append(linkerArgs, "-L"+getBDWGCLibDir(), "-lgc")
		// On macOS, LLVM prepends '_' to all symbol names in the object file,
		// so the linker must reference the symbol with the '_' prefix.
		entrySymbol := token.ZEUS_ENTRY_FUNCTION_NAME
		if runtime.GOOS == "darwin" {
			entrySymbol = "_" + entrySymbol
		}
		linkerArgs = append(linkerArgs, "-Wl,-e,"+entrySymbol)
		linkerArgs = append(linkerArgs, "-o", outputPath)
		linkerCmd = exec.Command(linker, linkerArgs...)
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
