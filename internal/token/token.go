package token

import (
	"fmt"
)

const MAIN_FUNCTION_NAME = "main"
const ZEUS_ENTRY_FUNCTION_NAME = "#_zeus_main"
const CONSTRUCTOR_METHOD_NAME = "constructor"
const FUNCTOR_CALL_METHOD_NAME = "__call__"
const THIS_KEYWORD = "this"

// Soft keywords: context-sensitive inside class bodies only, not reserved globally.
const GETTER_KEYWORD = "get"
const SETTER_KEYWORD = "set"
const READONLY_KEYWORD = "readonly"

type TokenType int

const (
	TokenTypeSemicolon TokenType = iota
	TokenTypeColon
	TokenTypeLeftParen
	TokenTypeRightParen
	TokenTypeLeftBrace
	TokenTypeRightBrace
	TokenTypeLeftBracket
	TokenTypeRightBracket
	TokenTypeComma
	TokenTypeDot
	// operators
	operator_beg
	TokenTypePlus
	TokenTypeMinus
	TokenTypeStar
	TokenTypeSlash
	TokenTypePercent
	TokenTypeDoubleStar
	TokenTypeEqual
	TokenTypeBang
	TokenTypeEqualEqual
	TokenTypeBangEqual
	TokenTypeGreaterThan
	TokenTypeGreaterThanEqual
	TokenTypeLessThan
	TokenTypeLessThanEqual
	TokenTypeAmpAmp
	TokenTypePipePipe
	TokenTypePlusEqual
	TokenTypeMinusEqual
	TokenTypeStarEqual
	TokenTypeSlashEqual
	TokenTypePercentEqual
	TokenTypePlusPlus
	TokenTypeMinusMinus
	TokenTypeDoubleStarEqual
	// bitwise operators
	TokenTypeBitwiseAnd      // &
	TokenTypeBitwiseOr       // |
	TokenTypeBitwiseXor      // ^
	TokenTypeBitwiseNot      // ~
	TokenTypeLeftShift       // <<
	TokenTypeRightShift      // >>
	TokenTypeBitwiseAndEqual // &=
	TokenTypeBitwiseOrEqual  // |=
	TokenTypeBitwiseXorEqual // ^=
	TokenTypeLeftShiftEqual  // <<=
	TokenTypeRightShiftEqual // >>=
	TokenTypeQuestion        // ?
	TokenTypeArrow           // =>
	TokenTypeEllipsis        // ...
	operator_end
	// literals
	literal_beg
	TokenTypeNumber
	TokenTypeIdentifier
	TokenTypeString
	TokenTypeChar
	literal_end
	// keywords
	keyword_beg
	TokenTypeLet
	TokenTypeConst
	TokenTypeFunction
	TokenTypeReturn
	TokenTypeIf
	TokenTypeElse
	TokenTypeWhile
	TokenTypeFor
	TokenTypeTrue
	TokenTypeFalse
	TokenTypeImport
	TokenTypeExport
	TokenTypeFrom
	TokenTypeClass
	TokenTypePrivate
	TokenTypePublic
	TokenTypeProtected
	TokenTypeNew      // new keyword
	TokenTypeTry      // try keyword
	TokenTypeCatch    // catch keyword
	TokenTypeThrow    // throw keyword
	TokenTypeExtends  // extends keyword for class inheritance
	TokenTypeBreak    // break keyword
	TokenTypeContinue // continue keyword
	keyword_end
	// data types
	datatype_beg
	TokenTypeVoid
	// signed integer types
	TokenTypeInt8
	TokenTypeInt16
	TokenTypeInt32
	TokenTypeInt64
	// unsigned integer types
	TokenTypeUInt8
	TokenTypeUInt16
	TokenTypeUInt32
	TokenTypeUInt64
	// floating point types
	TokenTypeFloat32
	TokenTypeFloat64
	// boolean type
	TokenTypeBoolean
	// null type
	TokenTypeNull
	datatype_end
	// template literal markers
	TokenTypeTemplateLiteralStr       // static segment (always emitted, possibly "")
	TokenTypeTemplateLiteralExprStart // ${
	TokenTypeTemplateLiteralExprEnd   // } closing interpolation
	TokenTypeTemplateLiteralEnd       // closing backtick
	// EOF
	TokenTypeEOF
)

func (t TokenType) String() string {
	switch t {
	case TokenTypeSemicolon:
		return ";"
	case TokenTypeColon:
		return ":"
	case TokenTypeLeftParen:
		return "("
	case TokenTypeRightParen:
		return ")"
	case TokenTypeLeftBrace:
		return "{"
	case TokenTypeRightBrace:
		return "}"
	case TokenTypeLeftBracket:
		return "["
	case TokenTypeRightBracket:
		return "]"
	case TokenTypeComma:
		return ","
	case TokenTypeDot:
		return "."
	case TokenTypePlus:
		return "+"
	case TokenTypeMinus:
		return "-"
	case TokenTypeStar:
		return "*"
	case TokenTypeSlash:
		return "/"
	case TokenTypePercent:
		return "%"
	case TokenTypeDoubleStar:
		return "**"
	case TokenTypeEqual:
		return "="
	case TokenTypeBang:
		return "!"
	case TokenTypeEqualEqual:
		return "=="
	case TokenTypeBangEqual:
		return "!="
	case TokenTypeGreaterThan:
		return ">"
	case TokenTypeGreaterThanEqual:
		return ">="
	case TokenTypeLessThan:
		return "<"
	case TokenTypeLessThanEqual:
		return "<="
	case TokenTypeAmpAmp:
		return "&&"
	case TokenTypePipePipe:
		return "||"
	case TokenTypePlusEqual:
		return "+="
	case TokenTypeMinusEqual:
		return "-="
	case TokenTypeStarEqual:
		return "*="
	case TokenTypeSlashEqual:
		return "/="
	case TokenTypePercentEqual:
		return "%="
	case TokenTypePlusPlus:
		return "++"
	case TokenTypeMinusMinus:
		return "--"
	case TokenTypeDoubleStarEqual:
		return "**="
	case TokenTypeBitwiseAnd:
		return "&"
	case TokenTypeBitwiseOr:
		return "|"
	case TokenTypeBitwiseXor:
		return "^"
	case TokenTypeBitwiseNot:
		return "~"
	case TokenTypeLeftShift:
		return "<<"
	case TokenTypeRightShift:
		return ">>"
	case TokenTypeBitwiseAndEqual:
		return "&="
	case TokenTypeBitwiseOrEqual:
		return "|="
	case TokenTypeBitwiseXorEqual:
		return "^="
	case TokenTypeLeftShiftEqual:
		return "<<="
	case TokenTypeRightShiftEqual:
		return ">>="
	case TokenTypeQuestion:
		return "?"
	case TokenTypeArrow:
		return "=>"
	case TokenTypeEllipsis:
		return "..."
	case TokenTypeNumber:
		return "number"
	case TokenTypeString:
		return "string"
	case TokenTypeIdentifier:
		return "identifier"
	case TokenTypeLet:
		return "let"
	case TokenTypeConst:
		return "const"
	case TokenTypeFunction:
		return "function"
	case TokenTypeReturn:
		return "return"
	case TokenTypeIf:
		return "if"
	case TokenTypeElse:
		return "else"
	case TokenTypeWhile:
		return "while"
	case TokenTypeFor:
		return "for"
	case TokenTypeTrue:
		return "true"
	case TokenTypeFalse:
		return "false"
	case TokenTypeVoid:
		return "void"
	case TokenTypeImport:
		return "import"
	case TokenTypeExport:
		return "export"
	case TokenTypeFrom:
		return "from"
	case TokenTypeInt8:
		return "i8"
	case TokenTypeInt16:
		return "i16"
	case TokenTypeInt32:
		return "i32"
	case TokenTypeInt64:
		return "i64"
	case TokenTypeUInt8:
		return "u8"
	case TokenTypeUInt16:
		return "u16"
	case TokenTypeUInt32:
		return "u32"
	case TokenTypeUInt64:
		return "u64"
	case TokenTypeFloat32:
		return "f32"
	case TokenTypeFloat64:
		return "f64"
	case TokenTypeBoolean:
		return "boolean"
	case TokenTypeClass:
		return "class"
	case TokenTypePrivate:
		return "private"
	case TokenTypePublic:
		return "public"
	case TokenTypeProtected:
		return "protected"
	case TokenTypeNew:
		return "new"
	case TokenTypeTry:
		return "try"
	case TokenTypeCatch:
		return "catch"
	case TokenTypeThrow:
		return "throw"
	case TokenTypeExtends:
		return "extends"
	case TokenTypeBreak:
		return "break"
	case TokenTypeContinue:
		return "continue"
	case TokenTypeNull:
		return "null"
	case TokenTypeChar:
		return "char"
	case TokenTypeTemplateLiteralStr:
		return "template-literal-str"
	case TokenTypeTemplateLiteralExprStart:
		return "${"
	case TokenTypeTemplateLiteralExprEnd:
		return "template-literal-expr-end"
	case TokenTypeTemplateLiteralEnd:
		return "template-literal-end"
	case TokenTypeEOF:
		return "EOF"
	}
	panic(fmt.Sprintf("could not convert token type to string: %d", t))
}

var Keywords = map[string]TokenType{
	TokenTypeLet.String():       TokenTypeLet,
	TokenTypeConst.String():     TokenTypeConst,
	TokenTypeFunction.String():  TokenTypeFunction,
	TokenTypeReturn.String():    TokenTypeReturn,
	TokenTypeIf.String():        TokenTypeIf,
	TokenTypeElse.String():      TokenTypeElse,
	TokenTypeWhile.String():     TokenTypeWhile,
	TokenTypeFor.String():       TokenTypeFor,
	TokenTypeTrue.String():      TokenTypeTrue,
	TokenTypeFalse.String():     TokenTypeFalse,
	TokenTypeImport.String():    TokenTypeImport,
	TokenTypeExport.String():    TokenTypeExport,
	TokenTypeFrom.String():      TokenTypeFrom,
	TokenTypeClass.String():     TokenTypeClass,
	TokenTypePrivate.String():   TokenTypePrivate,
	TokenTypePublic.String():    TokenTypePublic,
	TokenTypeProtected.String(): TokenTypeProtected,
	TokenTypeNew.String():       TokenTypeNew,
	TokenTypeTry.String():       TokenTypeTry,
	TokenTypeCatch.String():     TokenTypeCatch,
	TokenTypeThrow.String():     TokenTypeThrow,
	TokenTypeExtends.String():   TokenTypeExtends,
	TokenTypeNull.String():      TokenTypeNull,
	TokenTypeBreak.String():     TokenTypeBreak,
	TokenTypeContinue.String():  TokenTypeContinue,
}

var DataTypes = map[string]TokenType{
	TokenTypeInt8.String():    TokenTypeInt8,
	TokenTypeInt16.String():   TokenTypeInt16,
	TokenTypeInt32.String():   TokenTypeInt32,
	TokenTypeInt64.String():   TokenTypeInt64,
	TokenTypeUInt8.String():   TokenTypeUInt8,
	TokenTypeUInt16.String():  TokenTypeUInt16,
	TokenTypeUInt32.String():  TokenTypeUInt32,
	TokenTypeUInt64.String():  TokenTypeUInt64,
	TokenTypeFloat32.String(): TokenTypeFloat32,
	TokenTypeFloat64.String(): TokenTypeFloat64,
	TokenTypeBoolean.String(): TokenTypeBoolean,
	TokenTypeVoid.String():    TokenTypeVoid,
	TokenTypeNull.String():    TokenTypeNull,
}

type Position struct {
	Line   int
	Column int
}

func NewPosition(line int, column int) *Position {
	return &Position{Line: line, Column: column}
}

func (p *Position) String() string {
	return fmt.Sprintf("{Line: %d, Column: %d}", p.Line, p.Column)
}

func (p *Position) Equal(other *Position) bool {
	return p.Line == other.Line && p.Column == other.Column
}

type Span struct {
	Start Position
	End   Position
}

func (s *Span) String() string {
	return fmt.Sprintf("{Start: %s, End: %s}", &s.Start, &s.End)
}

func (s *Span) Equal(other *Span) bool {
	return s.Start == other.Start && s.End == other.End
}

type Token struct {
	Type  TokenType
	Value string
	Span  *Span
}

func NewTokenWithValue(tokenType TokenType, value string, span *Span) *Token {
	return &Token{Type: tokenType, Value: value, Span: span}
}

func NewToken(tokenType TokenType, span *Span) *Token {
	return &Token{Type: tokenType, Value: "", Span: span}
}

func NewSpan(start Position, end Position) *Span {
	return &Span{Start: start, End: end}
}

func (t *Token) String() string {
	if t.Value == "" {
		return fmt.Sprintf("{Type: %s, Span: %s}", t.Type, t.Span)
	}
	return fmt.Sprintf("{Type: %s, Value: %s, Span: %s}", t.Type, t.Value, t.Span)
}

func (t *Token) Equal(other *Token) bool {
	return t.Type == other.Type && t.Value == other.Value && t.Span.Equal(other.Span)
}

func (t *Token) IsDataType() bool {
	return t.Type >= datatype_beg && t.Type <= datatype_end
}
