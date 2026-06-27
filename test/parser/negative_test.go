package parser_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/test_utils"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

// parseErr builds a single-error expected slice.
func parseErr(msg string, line, colStart, colEnd int) []*zeus_error.ZeusError {
	return []*zeus_error.ZeusError{
		zeus_error.NewZeusError(
			zeus_error.ErrorSeverityError,
			msg,
			token.NewSpan(
				token.Position{Line: line, Column: colStart},
				token.Position{Line: line, Column: colEnd},
			),
		),
	}
}

func runNegativeTest(t *testing.T, input string, expectedErrors []*zeus_error.ZeusError) {
	t.Helper()
	l := lexer.NewLexer(input)
	tokens, _ := l.Lex()
	p := parser.NewParser(tokens)
	_, errors := p.ParseProgram()
	test_utils.CompareZeusErrors(t, errors, expectedErrors)
}

func TestNegativeIfStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "missing ( after if",
			input:  "if x > 0 { }",
			errors: parseErr("expected ( after if, but found identifier", 1, 4, 4),
		},
		{
			name:   "missing ) after if condition",
			input:  "if (x > 0 { }",
			errors: parseErr("expected ) after if condition, but found {", 1, 11, 11),
		},
		{
			// ASI inserts ; after ) before EOF, so the body parse finds ; instead of EOF.
			name:   "missing body after if condition (EOF)",
			input:  "if (x > 0) ",
			errors: parseErr("expected expression, but found ;", 1, 10, 10),
		},
		{
			name:   "else without preceding if",
			input:  "else { }",
			errors: parseErr("else statement must be preceded by an if statement", 1, 1, 4),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeWhileStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "missing expression for condition",
			input:  "while { }",
			errors: parseErr("expected expression in while condition, but found {", 1, 7, 7),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeForStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "missing ( after for",
			input:  "for x; x < 10; x++ { }",
			errors: parseErr("expected ( after for, but found identifier", 1, 5, 5),
		},
		{
			// Use an expression init (not let) so the ; is clearly missing after the init expr.
			// "for (x < 10 i++) { }" — after parsing `x < 10`, sees `i` instead of `;`.
			name:   "missing ; after for loop initializer",
			input:  "for (x < 10 i++) { }",
			errors: parseErr("expected ; after for loop initializer, but found identifier", 1, 13, 13),
		},
		{
			// "for (let i: i8 = 0; x < 10 i++) { }" — sees `i` instead of `;` after condition.
			name:   "missing ; after for loop condition",
			input:  "for (let i: i8 = 0; x < 10 i++) { }",
			errors: parseErr("expected ; after for loop condition, but found identifier", 1, 28, 28),
		},
		{
			// "for (let i: i8 = 0; i < 10; i++ { }" — sees `{` instead of `)` after update.
			name:   "missing ) after for loop update",
			input:  "for (let i: i8 = 0; i < 10; i++ { }",
			errors: parseErr("expected ) after for loop update, but found {", 1, 33, 33),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeBlockStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "missing closing brace",
			input:  "{ let x: i8 = 5;",
			errors: parseErr("expected } to close block", 1, 17, 17),
		},
		{
			// Function body missing `{` — produces "expected { to begin block, but found <id>".
			name:   "missing opening brace for function body",
			input:  "function f(): void x;",
			errors: parseErr("expected { to begin block, but found identifier", 1, 20, 20),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeVarDecl(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "let with no declarations",
			input:  "let;",
			errors: parseErr("expected at least one variable declaration", 1, 4, 4),
		},
		{
			name:   "missing data type after colon",
			input:  "let x: = 1;",
			errors: parseErr("expected data type in variable declaration, but found =", 1, 8, 8),
		},
		{
			name:   "missing initializer expression with explicit type",
			input:  "let x: i8 =;",
			errors: parseErr("expected expression for variable initializer, but found ;", 1, 12, 12),
		},
		{
			name:   "missing initializer expression without type",
			input:  "let x =;",
			errors: parseErr("expected expression for variable initializer, but found ;", 1, 8, 8),
		},
		{
			name:   "const with no declarations",
			input:  "const;",
			errors: parseErr("expected at least one variable declaration", 1, 6, 6),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeFunctionDeclaration(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "missing function name",
			input:  "function",
			errors: parseErr("expected identifier for function name", 1, 9, 9),
		},
		{
			name:   "missing ( after function name",
			input:  "function name",
			errors: parseErr("expected ( after function name, but found ;", 1, 13, 13),
		},
		{
			name:   "missing ) after function params",
			input:  "function name(",
			errors: parseErr("expected ) after function parameters", 1, 15, 15),
		},
		{
			name:   "missing : in parameter",
			input:  "function name(a)",
			errors: parseErr("expected : after identifier in function parameter, but found )", 1, 16, 16),
		},
		{
			name:   "missing data type in parameter",
			input:  "function name(a:)",
			errors: parseErr("expected data type in function parameter, but found )", 1, 17, 17),
		},
		{
			name:   "missing : after parameters for return type",
			input:  "function name(a: i8) {}",
			errors: parseErr("expected : after function parameters, but found {", 1, 22, 22),
		},
		{
			name:   "missing return type after :",
			input:  "function name(a: i8): {}",
			errors: parseErr("expected return type in function declaration, but found {", 1, 23, 23),
		},
		{
			name:   "missing { for function body",
			input:  "function name(a: i8): i8",
			errors: parseErr("expected { to begin block", 1, 25, 25),
		},
		{
			name:   "missing } to close function body",
			input:  "function name(a: i8): i8 {",
			errors: parseErr("expected } to close block", 1, 27, 27),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeClassDeclaration(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "missing class name",
			input:  "class",
			errors: parseErr("expected identifier class name", 1, 6, 6),
		},
		{
			// ASI inserts ; after the class name identifier before EOF.
			name:   "missing { after class name",
			input:  "class Foo",
			errors: parseErr("expected { after class name, but found ;", 1, 9, 9),
		},
		{
			// Class body close uses its own "after class members" context, not "to close block".
			name:   "missing } to close class body",
			input:  "class Foo {",
			errors: parseErr("expected } after class members", 1, 12, 12),
		},
		{
			name:   "missing : in class property",
			input:  "class Foo { x }",
			errors: parseErr("expected : after identifier in class property, but found }", 1, 15, 15),
		},
		{
			name:   "missing { after class name with extends",
			input:  "class Foo extends Bar",
			errors: parseErr("expected { after class name, but found ;", 1, 21, 21),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeImportStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "missing { after import",
			input:  "import add } from \"math\";",
			errors: parseErr("expected { after import, but found identifier", 1, 8, 10),
		},
		{
			name:   "missing } after import list",
			input:  "import { add from \"math\";",
			errors: parseErr("expected identifier import, but found from", 1, 14, 17),
		},
		{
			name:   "missing from keyword",
			input:  "import { add } \"math\";",
			errors: parseErr("expected from after imports, but found string", 1, 16, 21),
		},
		{
			name:   "missing source string",
			input:  "import { add } from;",
			errors: parseErr("expected string after from, but found ;", 1, 20, 20),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeExportStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "export with identifier (not function or class)",
			input:  "export x;",
			errors: parseErr("export can only be used with function declaration", 1, 8, 8),
		},
		{
			name:   "export with literal",
			input:  "export 42;",
			errors: parseErr("export can only be used with function declaration", 1, 8, 9),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeTryCatchStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			// Error span is the tryBody block span (from { to }).
			name:   "try without any catch clause",
			input:  "try { }",
			errors: parseErr("try statement must have at least one catch clause", 1, 5, 7),
		},
		{
			name:   "catch missing ( after keyword",
			input:  "try { } catch { }",
			errors: parseErr("expected ( after catch, but found {", 1, 15, 15),
		},
		{
			name:   "catch missing : after error variable",
			input:  "try { } catch (e { }",
			errors: parseErr("expected : after error variable in catch clause, but found {", 1, 18, 18),
		},
		{
			name:   "catch missing ) after error type",
			input:  "try { } catch (e: Error { }",
			errors: parseErr("expected ) after error type in catch clause, but found {", 1, 25, 25),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeThrowStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "throw with no expression",
			input:  "throw;",
			errors: parseErr("expected expression in throw statement, but found ;", 1, 6, 6),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}

func TestNegativeExpressions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors []*zeus_error.ZeusError
	}{
		{
			name:   "double unary operator",
			input:  "1 + + 2;",
			errors: parseErr("expected expression, but found +", 1, 5, 5),
		},
		{
			name:   "unclosed parenthesized expression",
			input:  "(1 + 2;",
			errors: parseErr("expected ) to close grouped expression, but found ;", 1, 7, 7),
		},
		{
			name:   "empty expression statement",
			input:  ";",
			errors: parseErr("expected expression, but found ;", 1, 1, 1),
		},
		{
			name:   "missing ) after function call arguments",
			input:  "add(1, 2;",
			errors: parseErr("expected ) after function call arguments, but found ;", 1, 9, 9),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNegativeTest(t, test.input, test.errors)
		})
	}
}
