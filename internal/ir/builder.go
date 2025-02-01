package ir

import "github.com/ameerthehacker/zeus/internal/symbol_table"

type IRBuilder struct {
	symbolTable *symbol_table.SymbolTable[ValueType]
}

func NewIRBuilder() *IRBuilder {
	return &IRBuilder{
		symbolTable: symbol_table.NewSymbolTable[ValueType](),
	}
}
