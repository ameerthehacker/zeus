package formatter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/formatter"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/token"
)

// parseSource lexes and parses source, returning the program and its comments, or
// ok=false if lexing/parsing failed.
func parseSource(t *testing.T, source string) (*ast.ProgramNode, []*token.Comment, bool) {
	t.Helper()
	lex := lexer.NewLexer(source)
	tokens, lexErrs := lex.Lex()
	if len(lexErrs) > 0 {
		return nil, nil, false
	}
	p := parser.NewParser(tokens)
	program, parseErrs := p.ParseProgram()
	if len(parseErrs) > 0 {
		return nil, nil, false
	}
	return program, lex.Comments(), true
}

func format(t *testing.T, source string) string {
	t.Helper()
	program, comments, ok := parseSource(t, source)
	if !ok {
		t.Fatalf("failed to parse source:\n%s", source)
	}
	// The golden/comment cases below verify spacing, comments, and blank-line handling — details
	// orthogonal to semicolons. Format them with Semicolons on so the fixtures stay stable and
	// readable; the no-semicolon default is covered by TestFormatNoSemicolons and the whole-suite
	// TestIdempotentAndStable (which uses DefaultOptions).
	opts := formatter.DefaultOptions()
	opts.Semicolons = true
	return formatter.Format(program, comments, opts)
}

func TestFormatNoSemicolons(t *testing.T) {
	// DefaultOptions has Semicolons off. Because Zeus's ASI inserts a `;` at a newline after a
	// value OR a type keyword, terminators are omitted everywhere they are redundant — statements,
	// class fields, and interface members alike. Only keyword-terminated statements (a bare
	// `return`, `break`, `continue`) keep an explicit `;`, since ASI never fires after a keyword.
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "value- and type-terminated statements drop semicolons",
			input: "function f(): i32 { let x: i32 = 5; return x + 1; }",
			expected: `function f(): i32 {
    let x: i32 = 5
    return x + 1
}
`,
		},
		{
			name:  "class fields and method bodies drop semicolons",
			input: "class C { public c: i32; constructor(v: i32) { this.c = v; } }",
			expected: `class C {
    public c: i32
    constructor(v: i32) {
        this.c = v
    }
}
`,
		},
		{
			name:  "interface members drop semicolons",
			input: "interface I { readonly x: i32; foo(): i32; }",
			expected: `interface I {
    readonly x: i32
    foo(): i32
}
`,
		},
		{
			name:  "keyword-terminated statements keep their semicolon",
			input: "function f(): void { while (true) { break; } return; }",
			expected: `function f(): void {
    while (true) {
        break;
    }
    return;
}
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, comments, ok := parseSource(t, tc.input)
			if !ok {
				t.Fatalf("failed to parse: %s", tc.input)
			}
			got := formatter.Format(program, comments, formatter.DefaultOptions())
			if got != tc.expected {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.expected)
			}
		})
	}
}

func TestFormatGolden(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "var decl spacing",
			input:    "let   x:i32=5;",
			expected: "let x: i32 = 5;\n",
		},
		{
			name:     "inferred var decl keeps no type",
			input:    "let x=5;",
			expected: "let x = 5;\n",
		},
		{
			name:     "boolean literal",
			input:    "let b:boolean=true;",
			expected: "let b: boolean = true;\n",
		},
		{
			name:     "binary operator spacing",
			input:    "let x:i32=1+2*3;",
			expected: "let x: i32 = 1 + 2 * 3;\n",
		},
		{
			name:  "function",
			input: "function add(a:i32,b:i32):i32{return a+b;}",
			expected: `function add(a: i32, b: i32): i32 {
    return a + b;
}
`,
		},
		{
			name:  "if else",
			input: "if(x>0){return 1;}else{return 2;}",
			expected: `if (x > 0) {
    return 1;
} else {
    return 2;
}
`,
		},
		{
			name:  "else if chain",
			input: "if(x>0){return 1;}else if(x<0){return 2;}else{return 3;}",
			expected: `if (x > 0) {
    return 1;
} else if (x < 0) {
    return 2;
} else {
    return 3;
}
`,
		},
		{
			name:  "while loop",
			input: "while(i<10){i++;}",
			expected: `while (i < 10) {
    i++;
}
`,
		},
		{
			name:  "for loop",
			input: "for(let i:i32=0;i<10;i++){sum+=i;}",
			expected: `for (let i: i32 = 0; i < 10; i++) {
    sum += i;
}
`,
		},
		{
			name:  "class with constructor (blank lines follow source)",
			input: "class Point{public x:i32;constructor(x:i32){this.x=x;}}",
			expected: `class Point {
    public x: i32;
    constructor(x: i32) {
        this.x = x;
    }
}
`,
		},
		{
			name:  "class preserves source blank line between members",
			input: "class Point{\npublic x:i32;\n\nconstructor(x:i32){this.x=x;}\n}",
			expected: `class Point {
    public x: i32;

    constructor(x: i32) {
        this.x = x;
    }
}
`,
		},
		{
			name:     "import statement",
			input:    "import {Counter} from \"./counter\";",
			expected: "import { Counter } from \"./counter\";\n",
		},
		{
			name:     "array type",
			input:    "let xs:i32[]=new i32[10];",
			expected: "let xs: i32[] = new i32[10];\n",
		},
		{
			name:     "blank line collapse",
			input:    "let a:i32=1;\n\n\n\nlet b:i32=2;",
			expected: "let a: i32 = 1;\n\nlet b: i32 = 2;\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := format(t, tc.input)
			if got != tc.expected {
				t.Errorf("mismatch\n--- input ---\n%s\n--- expected ---\n%q\n--- got ---\n%q", tc.input, tc.expected, got)
			}
		})
	}
}

func TestFormatComments(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "leading own-line comment",
			input:    "// a leading comment\nlet x: i32 = 1;",
			expected: "// a leading comment\nlet x: i32 = 1;\n",
		},
		{
			name:     "trailing end-of-line comment",
			input:    "let x: i32 = 1; // trailing",
			expected: "let x: i32 = 1; // trailing\n",
		},
		{
			name:     "inline block comment",
			input:    "let x: i32 = 1; /* block */",
			expected: "let x: i32 = 1; /* block */\n",
		},
		{
			name:  "comment inside function body",
			input: "function main(): i32 {\n// inside\nreturn 0;\n}",
			expected: `function main(): i32 {
    // inside
    return 0;
}
`,
		},
		{
			name:  "dangling comment in body",
			input: "function main(): i32 {\n// only a comment\n}",
			expected: `function main(): i32 {
    // only a comment
}
`,
		},
		{
			name:  "jsdoc block comment re-indented",
			input: "/*\n * doc line\n */\nfunction main(): i32 { return 0; }",
			expected: `/*
 * doc line
 */
function main(): i32 {
    return 0;
}
`,
		},
		{
			name:  "comment before class member",
			input: "class C {\n// about x\npublic x: i32;\n}",
			expected: `class C {
    // about x
    public x: i32;
}
`,
		},
		{
			name:     "comment-only file is preserved",
			input:    "// just a comment",
			expected: "// just a comment\n",
		},
		{
			name:     "blank line around comment preserved",
			input:    "let a: i32 = 1;\n\n// note\nlet b: i32 = 2;",
			expected: "let a: i32 = 1;\n\n// note\nlet b: i32 = 2;\n",
		},
		{
			name:  "consecutive leading comments",
			input: "// line one\n// line two\nlet x: i32 = 1;",
			expected: `// line one
// line two
let x: i32 = 1;
`,
		},
		{
			name:  "trailing comment on class member",
			input: "class C {\npublic x: i32; // the x\n}",
			expected: `class C {
    public x: i32; // the x
}
`,
		},
		{
			// Regression: CRLF line endings must not be counted as two lines and
			// inject spurious blank lines between statements.
			name:     "crlf endings do not add blank lines",
			input:    "let a: i32 = 1;\r\nlet b: i32 = 2;\r\nlet c: i32 = 3;",
			expected: "let a: i32 = 1;\nlet b: i32 = 2;\nlet c: i32 = 3;\n",
		},
		{
			// Regression: a non-JSDoc block comment keeps its interior layout verbatim.
			name:  "block comment table preserved verbatim",
			input: "/*\n  col1    col2\n  a       b\n*/\nlet x: i32 = 1;",
			expected: `/*
  col1    col2
  a       b
*/
let x: i32 = 1;
`,
		},
		{
			// Regression: with two statements on one line, the trailing comment
			// attaches to the last one, not the first.
			name:  "trailing comment attaches to last statement on line",
			input: "let a: i32 = 1; let b: i32 = 2; // c",
			expected: `let a: i32 = 1;
let b: i32 = 2; // c
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := format(t, tc.input)
			if got != tc.expected {
				t.Errorf("mismatch\n--- input ---\n%s\n--- expected ---\n%q\n--- got ---\n%q", tc.input, tc.expected, got)
			}
			// Formatting the formatted output must be a no-op (comments included).
			if again := format(t, got); again != got {
				t.Errorf("not idempotent\n--- first ---\n%q\n--- second ---\n%q", got, again)
			}
		})
	}
}

// TestCommentsKitchenSink runs a comment-heavy program through the formatter and
// asserts the result re-parses cleanly and is idempotent, exercising many comment
// positions at once (leading, trailing, dangling, block, and inside a class).
func TestCommentsKitchenSink(t *testing.T) {
	src := `// file header comment
import { Counter } from "./counter"; // an import

/* a block comment
   before the class */
class Widget {
    // a field
    public size: i32; // trailing on field

    // the constructor
    constructor(size: i32) {
        this.size = size; // assign
    }

    // an area method
    public area(): i32 {
        // compute it
        return this.size * this.size;
    }
}

// main entry point
function main(): i32 {
    let w: Widget = new Widget(4); // build one
    // check the area
    if (w.area() != 16) {
        return 1; // wrong
    }
    return 0;
    // trailing dangling comment
}`

	first := format(t, src)
	// format() already re-parses (parse-stability) and would fail otherwise.
	if second := format(t, first); second != first {
		t.Errorf("not idempotent\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	// Every comment token from the source must still be present after formatting.
	for _, want := range []string{
		"// file header comment", "// an import", "a block comment", "// a field",
		"// trailing on field", "// the constructor", "// assign", "// an area method",
		"// compute it", "// main entry point", "// build one", "// check the area",
		"// wrong", "// trailing dangling comment",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("formatted output is missing comment %q\n%s", want, first)
		}
	}
}

// TestIdempotentAndStable formats every parseable spec file twice and asserts
// (a) formatting is idempotent and (b) formatted output re-parses cleanly.
func TestIdempotentAndStable(t *testing.T) {
	specsDir := filepath.Join("..", "e2e", "specs")
	var files []string
	err := filepath.Walk(specsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".zs") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk specs: %s", err)
	}
	if len(files) == 0 {
		t.Fatalf("no .zs spec files found under %s", specsDir)
	}

	// Known v1 limitation: `new` allocations of function-type arrays
	// (`new ((x: i32) => i32)[]`) parse to a lossy/asymmetric AST that cannot be
	// reconstructed to round-tripping source without the original text.
	knownUnsupported := map[string]bool{
		"fn-type-array.zs": true,
	}

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			if knownUnsupported[filepath.Base(path)] {
				t.Skip("known v1 limitation: new of function-type arrays")
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %s", err)
			}

			program, comments, ok := parseSource(t, string(content))
			if !ok {
				t.Skipf("source does not parse cleanly; skipping")
				return
			}

			first := formatter.Format(program, comments, formatter.DefaultOptions())

			// (b) parse-stability: the formatted output must parse without errors.
			reparsed, reComments, ok := parseSource(t, first)
			if !ok {
				t.Fatalf("formatted output failed to re-parse:\n%s", first)
			}

			// (a) idempotency: formatting the formatted output is a no-op.
			second := formatter.Format(reparsed, reComments, formatter.DefaultOptions())
			if first != second {
				t.Errorf("not idempotent\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
		})
	}
}
