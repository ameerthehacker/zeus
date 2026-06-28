package logger

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/styles"
)

// zeusLexer is a Chroma lexer whose rules mirror zeus-vscode/syntaxes/zeus.tmLanguage.json.
// regexp2 (used internally by Chroma v2) supports lookaheads, so (?=\() is valid.
var zeusLexer = chroma.MustNewLexer(
	&chroma.Config{Name: "Zeus", Aliases: []string{"zeus", "zs"}, Filenames: []string{"*.zs"}},
	func() chroma.Rules {
		return chroma.Rules{
			"root": {
				{Pattern: `//[^\n]*`, Type: chroma.CommentSingle},
				{Pattern: `"`, Type: chroma.LiteralStringDouble, Mutator: chroma.Push("string")},
				{
					Pattern: chroma.Words(`\b`, `\b`,
						"if", "else", "return", "while", "new", "class",
						"private", "protected", "public", "this",
						"try", "throw", "catch", "finally",
						"import", "from", "export"),
					Type: chroma.Keyword,
				},
				{
					Pattern: chroma.Words(`\b`, `\b`, "function", "type", "extern", "let", "const"),
					Type:    chroma.KeywordDeclaration,
				},
				{
					Pattern: chroma.Words(`\b`, `\b`,
						"i8", "i16", "i32", "i64", "i128",
						"f16", "f32", "f64", "f128",
						"boolean", "void", "string", "null"),
					Type: chroma.KeywordType,
				},
				{Pattern: `\b(true|false)\b`, Type: chroma.LiteralStringBoolean},
				{Pattern: `0[xX][0-9a-fA-F_]+`, Type: chroma.LiteralNumberHex},
				{Pattern: `0[bB][01_]+`, Type: chroma.LiteralNumberBin},
				{Pattern: `0[oO][0-7_]+`, Type: chroma.LiteralNumberOct},
				{Pattern: `\d+\.\d+`, Type: chroma.LiteralNumberFloat},
				{Pattern: `\d+`, Type: chroma.LiteralNumberInteger},
				{Pattern: `\b[a-zA-Z_][a-zA-Z0-9_]*(?=\s*\()`, Type: chroma.NameFunction},
				{Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`, Type: chroma.NameOther},
				{Pattern: `\s+`, Type: chroma.Text},
				{Pattern: `\S`, Type: chroma.Text},
			},
			"string": {
				{Pattern: `\\.`, Type: chroma.LiteralStringEscape},
				{Pattern: `"`, Type: chroma.LiteralStringDouble, Mutator: chroma.Pop(1)},
				{Pattern: `[^"\\]+`, Type: chroma.LiteralStringDouble},
			},
		}
	},
)

var hlStyle = styles.Get("monokai")

func highlightZeusLine(line string) string {
	it, err := zeusLexer.Tokenise(nil, line)
	if err != nil {
		return line
	}
	var buf bytes.Buffer
	if err := formatters.TTY16m.Format(&buf, hlStyle, it); err != nil {
		return line
	}
	return strings.TrimRight(buf.String(), "\n")
}
