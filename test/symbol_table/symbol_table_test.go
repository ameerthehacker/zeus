package symbol_table_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/test_utils"
)

func TestSymbolTableDeclaration(t *testing.T) {
	symbol_table := symbol_table.NewSymbolTable[bool]()
	
	t.Run("a should be available in global scope", func(t *testing.T) {
		symbol_table.EnterScope()
		symbol_table.DeclareSymbol("a", true)
		_, ok := symbol_table.GetSymbol("a")
		test_utils.AssertEqual(t, ok, true)
	})

	t.Run("temp variable should be declared", func(t *testing.T) {
		temp_symbol_1 := symbol_table.DeclareTempSymbol(true)
		temp_symbol_2 := symbol_table.DeclareTempSymbol(true)
		_, ok1 := symbol_table.GetSymbol(temp_symbol_1)
		_, ok2 := symbol_table.GetSymbol(temp_symbol_2)

		test_utils.AssertEqual(t, temp_symbol_1, "%0")
		test_utils.AssertEqual(t, temp_symbol_2, "%1")
		test_utils.AssertEqual(t, ok1, true)
		test_utils.AssertEqual(t, ok2, true)
	})

	t.Run("a should not be available in local scope", func(t *testing.T) {
		symbol_table.EnterScope()
		_, ok := symbol_table.GetSymbolInCurrentScope("a")
		test_utils.AssertEqual(t, ok, false)
	})

	t.Run("b should be available in local scope", func(t *testing.T) {
		symbol_table.DeclareSymbol("b", true)
		_, ok := symbol_table.GetSymbolInCurrentScope("b")
		test_utils.AssertEqual(t, ok, true)
	})

	t.Run("temp variable count should restart from 0 in new scope", func(t *testing.T) {
		temp_symbol := symbol_table.DeclareTempSymbol(true)
		test_utils.AssertEqual(t, temp_symbol, "%0")
	})

	t.Run("b should be available in global scope", func(t *testing.T) {
		_, ok := symbol_table.GetSymbol("b")
		test_utils.AssertEqual(t, ok, true)
	})

	symbol_table.ExitScope()

	t.Run("b should not be available", func(t *testing.T) {
		_, okGlobal := symbol_table.GetSymbol("b")
		_, okLocal := symbol_table.GetSymbolInCurrentScope("b")
		test_utils.AssertEqual(t, okGlobal, false)
		test_utils.AssertEqual(t, okLocal, false)
	})

	symbol_table.EnterScope()
	symbol_table.DeclareSymbol("c", true)
	symbol_table.EnterScope()
	symbol_table.DeclareSymbol("d", true)
	symbol_table.ExitScope()
	symbol_table.ExitScope()
	symbol_table.ExitScope()

	// now symbol table is readonly and we treat it like snapshot
	symbol_table.SetReadonly(true)
	symbol_table.GoToGlobalScope()

	t.Run("a should be available", func(t *testing.T) {
		symbol_table.EnterScope()
		_, ok := symbol_table.GetSymbol("a")
		test_utils.AssertEqual(t, ok, true)
	})

	t.Run("b should not be available", func(t *testing.T) {
		_, ok := symbol_table.GetSymbol("b")
		test_utils.AssertEqual(t, ok, false)
	})

	t.Run("b should be available", func(t *testing.T) {
		symbol_table.EnterScope()
		_, ok := symbol_table.GetSymbol("b")
		test_utils.AssertEqual(t, ok, true)
	})

	t.Run("b should not be available", func(t *testing.T) {
		symbol_table.ExitScope()
		symbol_table.EnterScope()
		_, ok := symbol_table.GetSymbol("b")
		test_utils.AssertEqual(t, ok, false)
	})

	t.Run("c should be available", func(t *testing.T) {
		_, ok := symbol_table.GetSymbol("c")
		test_utils.AssertEqual(t, ok, true)
	})

	t.Run("d should be available", func(t *testing.T) {
		symbol_table.EnterScope()
		_, ok := symbol_table.GetSymbol("d")
		test_utils.AssertEqual(t, ok, true)
	})

	symbol_table.ExitScope()
	symbol_table.ExitScope()
	symbol_table.ExitScope()
}
