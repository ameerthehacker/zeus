package parser

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
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
)

var BinaryOperatorPrecedence = map[token.TokenType]int{
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
		params := []ast.ExprNode{}

		for {
			right := parser.parseExprOfPrecedence(0, false)
			params = append(params, right)
			if parser.peek().Type != token.TokenTypeComma {
				break
			}
			parser.consume()
		}

		closeParen := parser.consumeToken(token.TokenTypeRightParen)

		return &ast.FunctionCallExprNode{Callee: left, Params: params, Span: &token.Span{Start: left.GetSpan().Start, End: closeParen.Span.End}}
	}

	functionParselet := func(parser *Parser, functionKeyword *token.Token) ast.ExprNode {
		functionName := parser.consumeIdentifier("for function name")

		// consume the params
		params := []*ast.VarDeclNode{}
		parser.consumeToken(token.TokenTypeLeftParen, "after function name")
		for !parser.isEOF() && parser.peek().Type != token.TokenTypeRightParen {
			param := parser.parseVarDecl(false, ast.VarDeclTypeLet, "function parameter")
			params = append(params, param)
			parser.consumeOptionalToken(token.TokenTypeComma)
		}
		parser.consumeToken(token.TokenTypeRightParen, "after function parameters")
		// consume the return type
		parser.consumeToken(token.TokenTypeColon, "after function parameters")
		dataType := parser.consumeDataType("return type", "function declaration")
		// consume the body
		body := parser.parseBlockStmt()

		return &ast.FunctionDeclExprNode{Name: functionName, Params: params, Body: body, ReturnType: dataType, Span: &token.Span{Start: functionKeyword.Span.Start, End: body.GetSpan().End}}
	}

	prefixParselets := map[token.TokenType]func(parser *Parser, token *token.Token) ast.ExprNode{
		token.TokenTypeNumber: func(parser *Parser, token *token.Token) ast.ExprNode {
			return &ast.NumberExprNode{Value: token}
		},
		token.TokenTypeIdentifier: func(parser *Parser, token *token.Token) ast.ExprNode {
			return &ast.IdentifierExprNode{Name: token}
		},
		token.TokenTypeLeftParen: func(parser *Parser, openParen *token.Token) ast.ExprNode {
			expr := parser.parseExprOfPrecedence(0, false)
			closeParen := parser.consumeToken(token.TokenTypeRightParen)
			return &ast.GroupingExprNode{Expr: expr, Span: &token.Span{Start: openParen.Span.Start, End: closeParen.Span.End}}
		},
		token.TokenTypeFunction: functionParselet,
		token.TokenTypeMinus:    unaryOperatorParseLet,
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
	}

	return &Parser{tokens: tokens, current: 0, errors: []*zeus_error.ZeusError{}, prefixParselets: prefixParselets, infixParselets: infixParselets}
}

func (p *Parser) isEOF() bool {
	return p.tokens[p.current].Type == token.TokenTypeEOF
}

func (p *Parser) peek() *token.Token {
	return p.tokens[p.current]
}

func (p *Parser) consume() *token.Token {
	token := p.tokens[p.current]
	p.current++
	return token
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

func (p *Parser) consumeDataType(dataType string, cxt string) *token.Token {
	token := p.peek()
	if !token.IsDataType() {
		p.expectedButGotError(dataType, token, fmt.Sprintf("in %s", cxt))
	} else {
		p.consume()
	}
	return token
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
		message := fmt.Sprintf("expected %s%s but got %s", expected, extraInfoStr, token.Type)
		p.pushError(zeus_error.NewZeusError(zeus_error.ErrorSeverityError, message, token.Span))
	}
}

func (p *Parser) parseExprStmt() *ast.ExprStmtNode {
	expr := p.ParseExpr()

	switch expr.(type) {
	case *ast.FunctionDeclExprNode:
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

	return &ast.VarDeclNode{DeclType: declType, Identifier: identifier, Initializer: initializer, DataType: dataType}
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
