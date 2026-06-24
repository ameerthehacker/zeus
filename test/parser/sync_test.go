package parser_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
)

type syncTest struct {
	name       string
	input      string
	errorCount int
	stmtCount  int
}

func runSyncTest(t *testing.T, test syncTest) {
	t.Helper()
	l := lexer.NewLexer(test.input)
	tokens, _ := l.Lex()
	p := parser.NewParser(tokens)
	program, errors := p.ParseProgram()

	if len(errors) != test.errorCount {
		t.Errorf("expected %d error(s), got %d: %v", test.errorCount, len(errors), errors)
	}
	if len(program.Statements) != test.stmtCount {
		t.Errorf("expected %d statement(s), got %d", test.stmtCount, len(program.Statements))
	}
}

func TestSynchronizationRecovery(t *testing.T) {
	tests := []syncTest{
		{
			// One bad line (missing :), parser recovers and parses the next valid statement.
			name:       "recovery after var decl error",
			input:      "let x = 1;\nlet y: i8 = 2;",
			errorCount: 1,
			stmtCount:  1,
		},
		{
			// Two independent bad lines each produce exactly one error.
			name:       "two independent errors on different lines",
			input:      "let x = 1;\nlet z = 2;",
			errorCount: 2,
			stmtCount:  0,
		},
		{
			// One error followed by two valid statements.
			name:       "error then two valid statements",
			input:      "let x = 1;\nlet a: i8 = 2;\nlet b: i16 = 3;",
			errorCount: 1,
			stmtCount:  2,
		},
		{
			// With ASI, the newline after `5` acts as a semicolon — both statements parse successfully.
			name:       "newline after number literal is treated as semicolon",
			input:      "let x: i8 = 5\nlet y: i8 = 3;",
			errorCount: 0,
			stmtCount:  2,
		},
		{
			// Else without if is an error; synchronize stops before `{`, so the block `{ }`
			// and the subsequent `let` are both parsed — 2 recovered statements.
			name:       "else without if then valid statement",
			input:      "else { }\nlet x: i8 = 1;",
			errorCount: 1,
			stmtCount:  2,
		},
		{
			// Try without catch is an error; the next statement is recovered.
			name:       "try without catch then valid statement",
			input:      "try { }\nlet x: i8 = 1;",
			errorCount: 1,
			stmtCount:  1,
		},
		{
			// Error in the middle: good stmt, bad stmt, good stmt.
			name:       "error sandwiched between valid statements",
			input:      "let a: i8 = 1;\nlet b = 2;\nlet c: i8 = 3;",
			errorCount: 1,
			stmtCount:  2,
		},
		{
			// A single bad line with multiple missing tokens produces only 1 error
			// because the parser deduplicates errors on the same source line.
			name:       "same-line errors deduplicated",
			input:      "let x = ;",
			errorCount: 1,
			stmtCount:  0,
		},
		{
			// Completely valid program — no errors, all statements parsed.
			name:       "valid program has no errors",
			input:      "let a: i8 = 1;\nlet b: i32 = 2;\nlet c: f64 = 3.0;",
			errorCount: 0,
			stmtCount:  3,
		},
		{
			// Missing ( after if: synchronize consumes `x > 0` and stops before `{`,
			// so `{ }` and `let z` are both parsed — 2 recovered statements.
			name:       "if parse error then valid statement",
			input:      "if x > 0 { }\nlet z: i8 = 1;",
			errorCount: 1,
			stmtCount:  2,
		},
		{
			// Export error on line 1, then valid function on line 2.
			name:       "export error then valid statement",
			input:      "export x;\nlet y: i8 = 1;",
			errorCount: 1,
			stmtCount:  1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runSyncTest(t, test)
		})
	}
}

func TestASI(t *testing.T) {
	tests := []syncTest{
		{
			name:       "newline terminates var decl",
			input:      "let x: i8 = 5\nlet y: i8 = 3",
			errorCount: 0,
			stmtCount:  2,
		},
		{
			name:       "EOF without trailing newline or semicolon",
			input:      "let x: i8 = 5",
			errorCount: 0,
			stmtCount:  1,
		},
		{
			name:       "blank lines between statements",
			input:      "let x: i8 = 5\n\nlet y: i8 = 3",
			errorCount: 0,
			stmtCount:  2,
		},
		{
			name:       "mixed semicolons and newlines",
			input:      "let x: i8 = 5;\nlet y: i8 = 3",
			errorCount: 0,
			stmtCount:  2,
		},
		{
			name:       "newline terminates return statement",
			input:      "function f(): i8 {\nreturn 5\n}",
			errorCount: 0,
			stmtCount:  1,
		},
		{
			name:       "newline after postfix operator",
			input:      "let x: i8 = 5\nx++\nlet y: i8 = x",
			errorCount: 0,
			stmtCount:  3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runSyncTest(t, test)
		})
	}
}
