package codegen

import (
	"tinygo.org/x/go-llvm"
)

type Codegen struct {}

func NewCodegen() *Codegen {
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllAsmParsers()
	llvm.InitializeAllAsmPrinters()

	// Create a new LLVM context
	ctx := llvm.NewContext()
	defer ctx.Dispose()

	// Create a new module in this context
	mod := ctx.NewModule("example")
	defer mod.Dispose()
	
	// Print the LLVM IR
	mod.Dump()
	return &Codegen{}
}
