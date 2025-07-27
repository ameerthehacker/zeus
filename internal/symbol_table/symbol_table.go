package symbol_table

import (
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type Table[T any] struct {
	symbols map[string]T
	parent *Table[T]
	temp_symbol_count int
}

type SymbolTable[T any] struct {
	tables []*Table[T]
	current_scope *Table[T]
	next_scope_index int
	readonly bool
}

func NewSymbolTable[T any]() *SymbolTable[T] {
	return &SymbolTable[T]{
		tables: make([]*Table[T], 0),
		next_scope_index: 0,
		readonly: false,
	}
}

func (s *SymbolTable[T]) EnterScope() {
	if s.readonly {
		zeus_error.Assert(s.next_scope_index < len(s.tables), "trying to enter non existent scope")
		scope := s.tables[s.next_scope_index]
		s.current_scope = scope
	} else {
		new_scope := &Table[T]{
			symbols: make(map[string]T),
			parent:  s.current_scope,
			temp_symbol_count: 0,
		}
		s.tables = append(s.tables, new_scope)
		s.current_scope = new_scope
	}
	s.next_scope_index++
}

func (s *SymbolTable[T]) ExitScope() {
	zeus_error.Assert(s.current_scope != nil, "cannot exit global scope")
	s.current_scope = s.current_scope.parent
}

func (s *SymbolTable[T]) DeclareSymbol(name string, symbol T) {
	zeus_error.Assert(s.current_scope != nil, "no scope to declare symbol in")
	s.current_scope.symbols[name] = symbol
}

func (s *SymbolTable[T]) DeclareGlobalSymbol(name string, symbol T) {
	zeus_error.Assert(len(s.tables) > 0, "no global scope exists")
	s.tables[0].symbols[name] = symbol
}

func (s *SymbolTable[T]) GetSymbol(name string) (T, bool) {
	zeus_error.Assert(s.current_scope != nil, "no scope to get symbol in")
	current_scope := s.current_scope
	var zero T

	for current_scope != nil {
		symbol, ok := current_scope.symbols[name]
		if ok {
			return symbol, true
		}
		current_scope = current_scope.parent
	}
	
	return zero, false
}

func (s *SymbolTable[T]) IsGlobalScope() bool {
	return s.current_scope == s.tables[0]
}

func (s *SymbolTable[T]) SetReadonly(readonly bool) {
	s.readonly = readonly
}

func (s *SymbolTable[T]) GoToGlobalScope() {
	s.next_scope_index = 0
}

func (s *SymbolTable[T]) GetSymbolInCurrentScope(name string) (T, bool) {
	zeus_error.Assert(s.current_scope != nil, "no scope to get symbol in")
	symbol, ok := s.current_scope.symbols[name]
	return symbol, ok
}

// Walk iterates through all symbols in all scopes and calls the provided function for each symbol
func (s *SymbolTable[T]) Walk(fn func(name string, symbol T)) {
	for _, table := range s.tables {
		for name, symbol := range table.symbols {
			fn(name, symbol)
		}
	}
}
