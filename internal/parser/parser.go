package parser

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

type Parser struct {
	tokens          []*token.Token
	current         int
	prefixParselets map[token.TokenType]func(parser *Parser, token *token.Token) ast.ExprNode
	infixParselets  map[token.TokenType]func(parser *Parser, left ast.ExprNode, token *token.Token) ast.ExprNode
	errors          []*zeus_error.ZeusError
	lastSyncPos     *token.Position
}

const (
	UnaryOperatorPrecedence = 4
	NewOperatorPrecedence   = 5
)

var BinaryOperatorPrecedence = map[token.TokenType]int{
	token.TokenTypeDot:              6,
	token.TokenTypeLeftBracket:      6,
	token.TokenTypeLeftParen:        5,
	token.TokenTypeStar:             4,
	token.TokenTypeSlash:            4,
	token.TokenTypePlus:             3,
	token.TokenTypeMinus:            3,
	token.TokenTypeGreaterThan:      3,
	token.TokenTypeGreaterThanEqual: 3,
	token.TokenTypeLessThan:         3,
	token.TokenTypeLessThanEqual:    3,
	token.TokenTypeEqualEqual:       2,
	token.TokenTypeBangEqual:        2,
	token.TokenTypeEqual:            1,
}

var RightAssociativeOperators = map[token.TokenType]bool{
	token.TokenTypeEqual: true,
}

func getPrecedence(token *token.Token) int {
	precedence := 0

	if operatorPrecedence, ok := BinaryOperatorPrecedence[token.Type]; ok {
		precedence = operatorPrecedence
	}

	return precedence
}

func (p *Parser) parseFunctionSignatureAndBody(functionName *ast.IdentifierExprNode, isClassMethod bool) (*ast.IdentifierExprNode, []*ast.VarDeclNode, *ast.ValueTypeNode, *ast.BlockStmtNode) {
	fnType := "function"
	if isClassMethod {
		fnType = "method"
	}
	// consume the params
	params := []*ast.VarDeclNode{}
	p.consumeToken(token.TokenTypeLeftParen, "after function name")
	dataType := &ast.ValueTypeNode{ValueType: zeus_value.VoidType{}, Span: functionName.GetSpan()}
	for !p.isEOF() && p.peek().Type != token.TokenTypeRightParen {
		param := p.parseVarDecl(false, ast.VarDeclTypeLet, "function parameter")
		params = append(params, param)
		p.consumeOptionalToken(token.TokenTypeComma)
	}

	p.consumeToken(token.TokenTypeRightParen, fmt.Sprintf("after %s parameters", fnType))

	if isClassMethod && functionName.Name.Value == token.CONSTRUCTOR_METHOD_NAME {
		colon := p.consumeOptionalToken(token.TokenTypeColon)
		if colon != nil {
			dataType = p.consumeDataType("return type", fmt.Sprintf("%s declaration", fnType))
		}
	} else {
		p.consumeToken(token.TokenTypeColon, fmt.Sprintf("after %s parameters", fnType))
		// consume the return type
		dataType = p.consumeDataType("return type", fmt.Sprintf("%s declaration", fnType))
	}
	// consume the body
	body := p.parseBlockStmt()

	return functionName, params, dataType, body
}

func NewParser(tokens []*token.Token) *Parser {
	unaryOperatorParseLet := func(parser *Parser, token *token.Token) ast.ExprNode {
		expr := parser.parseExprOfPrecedence(UnaryOperatorPrecedence, false)
		return &ast.UnaryExprNode{Operator: token, Expr: expr}
	}

	binaryOperatorParseLet := func(parser *Parser, left ast.ExprNode, token *token.Token) ast.ExprNode {
		_, isRightAssociative := RightAssociativeOperators[token.Type]
		precedence := getPrecedence(token)

		// if the operator is right associative then we need to decrease the precedence
		if isRightAssociative {
			precedence--
		}

		right := parser.parseExprOfPrecedence(precedence, false)
		return &ast.BinaryExprNode{Left: left, Right: right, Operator: token}
	}

	functionCallParseLet := func(parser *Parser, left ast.ExprNode, openParen *token.Token) ast.ExprNode {
		callee, params, span := parser.parseFunctionCall(left)
		return &ast.FunctionCallExprNode{Callee: callee, Params: params, Span: span}
	}

	functionParselet := func(parser *Parser, functionKeyword *token.Token) ast.ExprNode {
		functionName := parser.consumeIdentifier("for function name")
		name, params, returnType, body := parser.parseFunctionSignatureAndBody(functionName, false)
		return &ast.FunctionDeclExprNode{
			Name:       name,
			Params:     params,
			Body:       body,
			ReturnType: returnType,
			Span:       &token.Span{Start: functionKeyword.Span.Start, End: body.GetSpan().End},
		}
	}

	classParselet := func(parser *Parser, classKeyword *token.Token) ast.ExprNode {
		className := parser.consumeIdentifier("class name")
		parser.consumeToken(token.TokenTypeLeftBrace, "after class name")

		methods := []*ast.ClassMethod{}
		properties := []*ast.ClassProperty{}

		for !parser.isEOF() && parser.peek().Type != token.TokenTypeRightBrace {
			accessModifier := parser.consumeAccessModifier()
			if parser.checkToken(token.TokenTypeIdentifier, "in class property or method declaration") && parser.lookahead(1, token.TokenTypeLeftParen) {
				methodName, params, returnType, body := parser.parseFunctionSignatureAndBody(parser.consumeIdentifier("method name"), true)
				spanStart := methodName.GetSpan().Start
				if accessModifier != nil {
					spanStart = accessModifier.Span.Start
				}
				methods = append(methods, &ast.ClassMethod{
					Name:           methodName,
					Params:         params,
					Body:           body,
					ReturnType:     returnType,
					AccessModifier: accessModifier,
					Span:           &token.Span{Start: spanStart, End: body.GetSpan().End},
				})
			} else {
				property := parser.parseVarDecl(false, ast.VarDeclTypeLet, "class property")
				spanStart := property.Identifier.GetSpan().Start
				if accessModifier != nil {
					spanStart = accessModifier.Span.Start
				}
				properties = append(properties, &ast.ClassProperty{
					Name:           property.Identifier,
					ValueType:      property.ValueType,
					AccessModifier: accessModifier,
					Span:           &token.Span{Start: spanStart, End: property.Identifier.GetSpan().End},
				})
				parser.consumeSemicolon()
			}
		}

		closeBrace := parser.consumeToken(token.TokenTypeRightBrace, "after class members")

		return &ast.ClassDeclExprNode{Name: className, Methods: methods, Properties: properties, Span: &token.Span{Start: classKeyword.Span.Start, End: closeBrace.Span.End}}
	}

	objectPropertyAccessParseLet := func(parser *Parser, left ast.ExprNode, dot *token.Token) ast.ExprNode {
		property := parser.consumeIdentifier("in object property access")
		return &ast.ObjectPropertyAccessExprNode{Object: left, Property: property, Span: &token.Span{Start: dot.Span.Start, End: property.GetSpan().End}}
	}

	prefixParselets := map[token.TokenType]func(parser *Parser, token *token.Token) ast.ExprNode{
		token.TokenTypeNumber: func(parser *Parser, token *token.Token) ast.ExprNode {
			return &ast.NumberExprNode{Value: token}
		},
		token.TokenTypeNull: func(parser *Parser, token *token.Token) ast.ExprNode {
			return &ast.NullExprNode{Span: token.Span}
		},
		token.TokenTypeTrue: func(parser *Parser, token *token.Token) ast.ExprNode {
			return &ast.BooleanExprNode{Value: token}
		},
		token.TokenTypeFalse: func(parser *Parser, token *token.Token) ast.ExprNode {
			return &ast.BooleanExprNode{Value: token}
		},
		token.TokenTypeIdentifier: func(parser *Parser, typeToken *token.Token) ast.ExprNode {
			return &ast.IdentifierExprNode{Name: typeToken}
		},
		token.TokenTypeLeftParen: func(parser *Parser, openParen *token.Token) ast.ExprNode {
			expr := parser.parseExprOfPrecedence(0, false)
			closeParen := parser.consumeToken(token.TokenTypeRightParen)
			return &ast.GroupingExprNode{Expr: expr, Span: &token.Span{Start: openParen.Span.Start, End: closeParen.Span.End}}
		},
		token.TokenTypeNew: func(parser *Parser, newKeyword *token.Token) ast.ExprNode {
			callee := parser.parseExprOfPrecedence(NewOperatorPrecedence, false)
			if ast.AsTypeExpression(callee) != nil {
				return &ast.NewExprNode{
					Callee: callee,
					Args:   []ast.ExprNode{},
					Span:   &token.Span{Start: newKeyword.Span.Start, End: callee.GetSpan().End},
				}
			} else {
				parser.consumeToken(token.TokenTypeLeftParen, "after class name in new expression")
				args, closeParen := parser.parseArgumentList()

				return &ast.NewExprNode{
					Callee: callee,
					Args:   args,
					Span:   &token.Span{Start: newKeyword.Span.Start, End: closeParen.Span.End},
				}
			}
		},
		token.TokenTypeFunction: functionParselet,
		token.TokenTypeMinus:    unaryOperatorParseLet,
		token.TokenTypeClass:    classParselet,
	}

	indexingExpressionParseLet := func(parser *Parser, left ast.ExprNode, openBracket *token.Token) ast.ExprNode {
		indexingMetadata := parser.consumeIndexingMetadata()
		return &ast.IndexingExprNode{Array: left, IndexingMeta: *indexingMetadata, Span: &token.Span{Start: openBracket.Span.Start, End: parser.peek().Span.End}}
	}

	infixParselets := map[token.TokenType]func(parser *Parser, left ast.ExprNode, token *token.Token) ast.ExprNode{
		token.TokenTypePlus:             binaryOperatorParseLet,
		token.TokenTypeMinus:            binaryOperatorParseLet,
		token.TokenTypeStar:             binaryOperatorParseLet,
		token.TokenTypeSlash:            binaryOperatorParseLet,
		token.TokenTypeEqualEqual:       binaryOperatorParseLet,
		token.TokenTypeBangEqual:        binaryOperatorParseLet,
		token.TokenTypeGreaterThan:      binaryOperatorParseLet,
		token.TokenTypeGreaterThanEqual: binaryOperatorParseLet,
		token.TokenTypeLessThan:         binaryOperatorParseLet,
		token.TokenTypeLessThanEqual:    binaryOperatorParseLet,
		token.TokenTypeEqual:            binaryOperatorParseLet,
		token.TokenTypeLeftParen:        functionCallParseLet,
		token.TokenTypeDot:              objectPropertyAccessParseLet,
		token.TokenTypeLeftBracket:      indexingExpressionParseLet,
	}

	return &Parser{tokens: tokens, current: 0, errors: []*zeus_error.ZeusError{}, prefixParselets: prefixParselets, infixParselets: infixParselets}
}

func (p *Parser) isEOF() bool {
	return p.tokens[p.current].Type == token.TokenTypeEOF
}

func (p *Parser) peek() *token.Token {
	return p.tokens[p.current]
}

func (p *Parser) checkToken(tokenType token.TokenType, extraInfo ...string) bool {
	if p.peek().Type == tokenType {
		return true
	}

	p.expectedButGotError(tokenType.String(), p.peek(), extraInfo...)

	return false
}

func (p *Parser) lookahead(n int, tokenType token.TokenType) bool {
	index := p.current + n
	if index >= len(p.tokens) {
		return false
	}
	return p.tokens[index].Type == tokenType
}

func (p *Parser) consumeAccessModifier() *token.Token {
	accessModifierTokens := []token.TokenType{token.TokenTypePublic, token.TokenTypePrivate, token.TokenTypeProtected}

	for _, tokenType := range accessModifierTokens {
		if p.peek().Type == tokenType {
			return p.consume()
		}
	}

	return nil
}

func (p *Parser) consume() *token.Token {
	token := p.tokens[p.current]
	p.current++
	return token
}

func (p *Parser) consumeIndexingMetadata() *ast.IndexingMeta {
	arrayMetadata := &ast.IndexingMeta{
		IndexingExprs: []ast.ExprNode{},
	}
	
	// Parse the first index expression (the '[' has already been consumed by the infix parselet)
	indexExpr := p.parseExprOfPrecedence(0, false)
	arrayMetadata.IndexingExprs = append(arrayMetadata.IndexingExprs, indexExpr)
	p.consumeToken(token.TokenTypeRightBracket)
	
	// Handle multi-dimensional array indexing (e.g., arr[0][1])
	for p.peek().Type == token.TokenTypeLeftBracket {
		p.consumeToken(token.TokenTypeLeftBracket)
		indexExpr := p.parseExprOfPrecedence(0, false)
		arrayMetadata.IndexingExprs = append(arrayMetadata.IndexingExprs, indexExpr)
		p.consumeToken(token.TokenTypeRightBracket)
	}

	return arrayMetadata
}

func (p *Parser) consumeToken(expectedTokenType token.TokenType, extraInfo ...string) *token.Token {
	token := p.tokens[p.current]
	if token.Type != expectedTokenType {
		p.expectedButGotError(expectedTokenType.String(), token, extraInfo...)
	} else {
		p.current++
	}
	return token
}

func (p *Parser) consumeOptionalToken(expectedTokenType token.TokenType) *token.Token {
	token := p.tokens[p.current]
	if token.Type != expectedTokenType {
		return nil
	}
	p.current++
	return token
}

func (p *Parser) consumeIdentifier(extraInfo ...string) *ast.IdentifierExprNode {
	token := p.consumeToken(token.TokenTypeIdentifier, extraInfo...)
	return &ast.IdentifierExprNode{Name: token}
}

func (p *Parser) consumeSemicolon(extraInfo ...string) {
	p.consumeToken(token.TokenTypeSemicolon, extraInfo...)
}

func (p *Parser) consumeDataType(dataType string, cxt string) *ast.ValueTypeNode {
	nextToken := p.peek()
	if nextToken.IsDataType() || nextToken.Type == token.TokenTypeIdentifier {
		p.consume()
	} else {
		p.expectedButGotError(dataType, nextToken, fmt.Sprintf("in %s", cxt))
	}

	valueType := zeus_value.ToValueType(nextToken)
	// check if the data type is an array
	for p.peek().Type == token.TokenTypeLeftBracket {
		p.consume()
		p.consumeToken(token.TokenTypeRightBracket, "in array definition")

		valueType = zeus_value.ArrayType{ElementType: valueType}
	}

	if zeus_value.IsArrayType(valueType) {
		return &ast.ValueTypeNode{ValueType: valueType, Span: nextToken.Span}
	}

	return &ast.ValueTypeNode{ValueType: valueType, Span: nextToken.Span}
}

func (p *Parser) pushError(err *zeus_error.ZeusError) {
	// if the last error is on the same line as the current error then it could be a side effect of the previous error
	// so we don't add the error
	if len(p.errors) > 0 {
		lastError := p.errors[len(p.errors)-1]
		if lastError.Span.Start.Line != err.Span.Start.Line {
			p.errors = append(p.errors, err)
		}
	} else {
		p.errors = append(p.errors, err)
	}

	panic(err)
}

func (p *Parser) expectedError(expected string, extraInfo ...string) {
	message := fmt.Sprintf("expected %s", expected)
	if len(extraInfo) > 0 {
		message += fmt.Sprintf(" %s", strings.Join(extraInfo, " "))
	}
	p.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, message, p.tokens[p.current].Span))
}

func (p *Parser) expectedButGotError(expected string, token *token.Token, extraInfo ...string) {
	if p.isEOF() {
		p.expectedError(expected, extraInfo...)
	} else {
		extraInfoStr := ""
		if len(extraInfo) > 0 {
			extraInfoStr = fmt.Sprintf(" %s", strings.Join(extraInfo, " "))
		}
		message := fmt.Sprintf("expected %s%s, but found %s", expected, extraInfoStr, token.Type)
		p.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, message, token.Span))
	}
}

func (p *Parser) parseExprStmt() *ast.ExprStmtNode {
	expr := p.ParseExpr()

	switch expr.(type) {
	case *ast.FunctionDeclExprNode:
	case *ast.ClassDeclExprNode:
	default:
		p.consumeSemicolon()
	}

	return &ast.ExprStmtNode{Expr: expr}
}

func (p *Parser) parseBlockStmt() *ast.BlockStmtNode {
	stmts := []ast.StmtNode{}
	openBrace := p.consumeToken(token.TokenTypeLeftBrace)

	for !p.isEOF() && p.peek().Type != token.TokenTypeRightBrace {
		stmt := p.ParseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}

	closeBrace := p.consumeToken(token.TokenTypeRightBrace)
	span := &token.Span{Start: openBrace.Span.Start, End: closeBrace.Span.End}

	return &ast.BlockStmtNode{Statements: stmts, Span: span}
}

func (p *Parser) parseReturnStmt() *ast.ReturnStmtNode {
	returnKeyword := p.consumeToken(token.TokenTypeReturn)
	expr := p.parseExprOfPrecedence(0, true)
	var span *token.Span
	p.consumeSemicolon()

	if expr != nil {
		span = &token.Span{Start: returnKeyword.Span.Start, End: expr.GetSpan().End}
	} else {
		span = &token.Span{Start: returnKeyword.Span.Start, End: returnKeyword.Span.End}
	}

	return &ast.ReturnStmtNode{Expr: expr, Span: span}
}

func (p *Parser) parseIfStmt() *ast.IfStmtNode {
	ifKeyword := p.consume()

	p.consumeToken(token.TokenTypeLeftParen, "after if")
	condition := p.parseExprOfPrecedence(0, false, "in if condition")
	p.consumeToken(token.TokenTypeRightParen, "after if condition")

	thenStmt := p.ParseStmt()

	if p.peek().Type == token.TokenTypeElse {
		p.consume()
		elseStmt := p.ParseStmt()
		span := &token.Span{Start: ifKeyword.Span.Start, End: elseStmt.GetSpan().End}
		return &ast.IfStmtNode{Condition: condition, ThenStmt: thenStmt, ElseStmt: elseStmt, Span: span}
	}

	span := &token.Span{Start: ifKeyword.Span.Start, End: thenStmt.GetSpan().End}

	return &ast.IfStmtNode{Condition: condition, ThenStmt: thenStmt, Span: span}
}

func (p *Parser) parseWhileStmt() *ast.WhileStmtNode {
	whileKeyword := p.consumeToken(token.TokenTypeWhile)
	condition := p.parseExprOfPrecedence(0, false, "in while condition")
	body := p.ParseStmt()
	span := &token.Span{Start: whileKeyword.Span.Start, End: body.GetSpan().End}

	return &ast.WhileStmtNode{Condition: condition, Body: body, Span: span}
}

func (p *Parser) parseVarDeclStmt() *ast.VarDeclStmtNode {
	var span *token.Span
	varDeclTypeToken := p.consume()
	varDeclType := ast.VarDeclTypeLet

	if varDeclTypeToken.Type == token.TokenTypeConst {
		varDeclType = ast.VarDeclTypeConst
	}

	decls := []ast.VarDeclNode{}

	for !p.isEOF() && p.peek().Type != token.TokenTypeSemicolon {
		decl := p.parseVarDecl(true, varDeclType, "variable declaration")
		decls = append(decls, *decl)
		p.consumeOptionalToken(token.TokenTypeComma)
	}

	if len(decls) == 0 {
		p.expectedError("atleast one variable declaration")
	}

	p.consumeSemicolon()

	if len(decls) > 0 {
		span = &token.Span{Start: varDeclTypeToken.Span.Start, End: decls[len(decls)-1].GetSpan().End}
	} else {
		span = &token.Span{Start: varDeclTypeToken.Span.Start, End: varDeclTypeToken.Span.End}
	}

	return &ast.VarDeclStmtNode{Decls: decls, Span: span}
}

func (p *Parser) parseVarDecl(allowInitializer bool, declType ast.VarDeclType, cxt string) *ast.VarDeclNode {
	var initializer ast.ExprNode

	identifier := p.consumeIdentifier(fmt.Sprintf("in %s", cxt))
	// parse the datatype
	p.consumeToken(token.TokenTypeColon, fmt.Sprintf("after identifier in %s", cxt))
	dataType := p.consumeDataType("data type", cxt)

	// check if the declaration has an initializer
	if allowInitializer && p.peek().Type == token.TokenTypeEqual {
		p.consume()
		initializer = p.ParseExpr("for variable initializer")
	}

	return &ast.VarDeclNode{DeclType: declType, Identifier: identifier, Initializer: initializer, ValueType: dataType}
}

func (p *Parser) parseArgumentList() ([]ast.ExprNode, *token.Token) {
	params := []ast.ExprNode{}

	for {
		right := p.parseExprOfPrecedence(0, true)
		if right == nil {
			break
		}
		params = append(params, right)
		if p.peek().Type != token.TokenTypeComma {
			break
		}
		p.consume()
	}

	closeParen := p.consumeToken(token.TokenTypeRightParen)
	return params, closeParen
}

func (p *Parser) parseFunctionCall(callee ast.ExprNode) (ast.ExprNode, []ast.ExprNode, *token.Span) {
	params, closeParen := p.parseArgumentList()
	span := &token.Span{Start: callee.GetSpan().Start, End: closeParen.Span.End}

	return callee, params, span
}

func (p *Parser) parseImportStmt() *ast.ImportStmtNode {
	importKeyword := p.consumeToken(token.TokenTypeImport)
	imports := []*ast.IdentifierExprNode{}

	p.consumeToken(token.TokenTypeLeftBrace, "after import")

	for !p.isEOF() && p.peek().Type != token.TokenTypeRightBrace {
		expr := p.consumeIdentifier("import")
		imports = append(imports, expr)
		p.consumeOptionalToken(token.TokenTypeComma)
	}

	p.consumeToken(token.TokenTypeRightBrace, "after imports")
	p.consumeToken(token.TokenTypeFrom, "after imports")
	source := p.consumeToken(token.TokenTypeString, "after from")
	p.consumeSemicolon()

	return &ast.ImportStmtNode{Imports: imports, Source: source, Span: &token.Span{Start: importKeyword.Span.Start, End: source.Span.End}}
}

func (p *Parser) parseExportStmt() *ast.ExportStmtNode {
	exportKeyword := p.consumeToken(token.TokenTypeExport)
	expr := p.ParseExpr()

	switch expr.(type) {
	case *ast.FunctionDeclExprNode:
	case *ast.ClassDeclExprNode:
	default:
		p.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "export can only be used with function declaration", expr.GetSpan()))
	}

	return &ast.ExportStmtNode{Expr: expr, Span: &token.Span{Start: exportKeyword.Span.Start, End: expr.GetSpan().End}}
}

// Synchronizes the parser by consuming tokens until it encounters a semicolon or right brace
// this helps in preventing errors that are side effects of a previous error
func (p *Parser) synchronize() {
	// this is to prevent infinite loops
	// if we are stuck on the same token then we consume it and return to paring again
	if p.lastSyncPos != nil && p.lastSyncPos == &p.peek().Span.Start {
		p.consume()
		return
	}

	stopAtTokens := map[token.TokenType]bool{
		token.TokenTypeLet:        true,
		token.TokenTypeConst:      true,
		token.TokenTypeFunction:   true,
		token.TokenTypeLeftParen:  true,
		token.TokenTypeRightParen: true,
		token.TokenTypeLeftBrace:  true,
		token.TokenTypeRightBrace: true,
		token.TokenTypeIf:         true,
	}
	stopAfterTokens := map[token.TokenType]bool{
		token.TokenTypeSemicolon: true,
		token.TokenTypeElse:      true,
	}

	canConsume := func(token *token.Token) bool {
		if p.isEOF() {
			return false
		}
		if _, ok := stopAtTokens[token.Type]; ok {
			return false
		}
		if _, ok := stopAfterTokens[token.Type]; ok {
			return false
		}
		return true
	}

	for canConsume(p.peek()) {
		p.consume()
	}
	// we consume tokens and not keywords
	if _, ok := stopAfterTokens[p.peek().Type]; ok {
		p.consume()
	}

	p.lastSyncPos = &p.peek().Span.Start
}

func (p *Parser) handlePanic() {
	if r := recover(); r != nil {
		switch r.(type) {
		case *zeus_error.ZeusError:
			p.synchronize()
		default:
			panic(r)
		}
	}
}

func (p *Parser) ParseStmt() ast.StmtNode {
	// handle panics and synchronize the parser
	defer p.handlePanic()

	switch p.peek().Type {
	case token.TokenTypeLet:
		fallthrough
	case token.TokenTypeConst:
		return p.parseVarDeclStmt()
	case token.TokenTypeLeftBrace:
		return p.parseBlockStmt()
	case token.TokenTypeReturn:
		return p.parseReturnStmt()
	case token.TokenTypeIf:
		return p.parseIfStmt()
	case token.TokenTypeElse:
		if p.lastSyncPos == nil {
			p.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, "else statement must be preceded by an if statement", p.peek().Span))
		} else {
			p.consume()
		}
		return nil
	case token.TokenTypeWhile:
		return p.parseWhileStmt()
	case token.TokenTypeImport:
		return p.parseImportStmt()
	case token.TokenTypeExport:
		return p.parseExportStmt()
	default:
		return p.parseExprStmt()
	}
}

func (p *Parser) GetErrors() []*zeus_error.ZeusError {
	return p.errors
}

func (p *Parser) Reset() {
	p.current = 0
	p.errors = []*zeus_error.ZeusError{}
}

func (p *Parser) ParseProgram() (*ast.ProgramNode, []*zeus_error.ZeusError) {
	p.errors = []*zeus_error.ZeusError{}
	stmts := []ast.StmtNode{}

	for !p.isEOF() {
		stmt := p.ParseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}

	return &ast.ProgramNode{Statements: stmts}, p.errors
}

func (p *Parser) ParseExpr(extraInfo ...string) ast.ExprNode {
	return p.parseExprOfPrecedence(0, false, extraInfo...)
}

func (p *Parser) parseExprOfPrecedence(precedence int, optional bool, extraInfo ...string) ast.ExprNode {
	var left ast.ExprNode

	if p.isEOF() {
		if !optional {
			p.expectedError("expression", extraInfo...)
		}
		return left
	}

	token := p.peek()
	prefixParselet, ok := p.prefixParselets[token.Type]

	if !ok {
		if !optional {
			p.expectedButGotError("expression", token, extraInfo...)
		}
		return left
	}

	p.consume()

	left = prefixParselet(p, token)

	for {
		token = p.peek()
		infixParselet, ok := p.infixParselets[token.Type]

		// if no infix available then we are done
		if !ok {
			return left
		}

		// we are done if the next operator has less precedence than the current operator
		nextOpPrecedence := getPrecedence(token)
		if nextOpPrecedence <= precedence {
			break
		}

		token = p.consume()
		left = infixParselet(p, left, token)
	}

	return left
}
