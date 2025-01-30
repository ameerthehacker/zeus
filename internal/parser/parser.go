package parser

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/error"
	"github.com/ameerthehacker/zeus/internal/token"
)
type Parser struct {
	tokens          []*token.Token
	current         int
	prefixParselets map[token.TokenType]func(parser *Parser, token *token.Token) ast.ExprNode
	infixParselets  map[token.TokenType]func(parser *Parser, left ast.ExprNode, token *token.Token) ast.ExprNode
	errors          []*error.ZeusError
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
		expr := parser.parseExprOfPrecedence(UnaryOperatorPrecedence)
		return &ast.UnaryExprNode{Operator: token, Expr: expr}
	}

	binaryOperatorParseLet := func(parser *Parser, left ast.ExprNode, token *token.Token) ast.ExprNode {
		_, isRightAssociative := RightAssociativeOperators[token.Type]
		precedence := getPrecedence(token)

		// if the operator is right associative then we need to decrease the precedence
		if isRightAssociative {
			precedence--
		}

		right := parser.parseExprOfPrecedence(precedence)
		return &ast.BinaryExprNode{Left: left, Right: right, Operator: token}
	}

	functionCallParseLet := func(parser *Parser, left ast.ExprNode, openParen *token.Token) ast.ExprNode {
		params := []ast.ExprNode{}
		
		for {
			right := parser.parseExprOfPrecedence(0)
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
		functionName := parser.consumeIdentifier()

		// consume the params
		params := []*ast.VarDeclNode{}
		parser.consumeToken(token.TokenTypeLeftParen)
		for !parser.isEOF() && parser.peek().Type != token.TokenTypeRightParen {
			param := parser.parseVarDecl(false, ast.VarDeclTypeLet)
			params = append(params, param)
			parser.consumeOptionalToken(token.TokenTypeComma)
		}
		parser.consumeToken(token.TokenTypeRightParen)
		// consume the return type
		parser.consumeToken(token.TokenTypeColon)
		dataType := parser.consumeDataType()
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
			expr := parser.parseExprOfPrecedence(0)
			closeParen := parser.consumeToken(token.TokenTypeRightParen)
			return &ast.GroupingExprNode{Expr: expr, Span: &token.Span{Start: openParen.Span.Start, End: closeParen.Span.End}}
		},
		token.TokenTypeFunction: functionParselet,
		token.TokenTypeMinus: unaryOperatorParseLet,
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

	return &Parser{tokens: tokens, current: 0, errors: []*error.ZeusError{}, prefixParselets: prefixParselets, infixParselets: infixParselets}
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

func (p *Parser) consumeToken(expectedTokenType token.TokenType) *token.Token {
	token := p.tokens[p.current]
	if token.Type != expectedTokenType {
		p.expectedButGotError(expectedTokenType.String(), token)
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

func (p *Parser) consumeIdentifier() *ast.IdentifierExprNode {
	token := p.consumeToken(token.TokenTypeIdentifier)
	return &ast.IdentifierExprNode{Name: token}
}

func (p* Parser) consumeSemicolon() {
	p.consumeToken(token.TokenTypeSemicolon)
}

func (p *Parser) consumeDataType() *token.Token {
	token := p.consume()
	if !token.IsDataType() {
		p.expectedButGotError("data type", token)
	}
	return token
}

func (p *Parser) pushError(err *error.ZeusError) {
	p.errors = append(p.errors, err)
}

func (p *Parser) expectedError(expected string) {
	message := fmt.Sprintf("expected %s", expected)
	p.pushError(error.NewZeusError(error.ErrorSeverityError, message, p.tokens[p.current].Span))
}

func (p *Parser) expectedButGotError(expected string, token *token.Token) {
	if p.isEOF() {
		p.expectedError(expected)
	} else {
		message := fmt.Sprintf("expected %s but got %s", expected, token.Type)
		p.pushError(error.NewZeusError(error.ErrorSeverityError, message, token.Span))
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
		stmts = append(stmts, stmt)
	}

	closeBrace := p.consumeToken(token.TokenTypeRightBrace)
	span := &token.Span{Start: openBrace.Span.Start, End: closeBrace.Span.End}

	return &ast.BlockStmtNode{Statements: stmts, Span: span}
}

func (p *Parser) parseReturnStmt() *ast.ReturnStmtNode {
	returnKeyword := p.consumeToken(token.TokenTypeReturn)
	expr := p.ParseExpr()
	p.consumeSemicolon()

	return &ast.ReturnStmtNode{Expr: expr, Span: &token.Span{Start: returnKeyword.Span.Start, End: expr.GetSpan().End}}
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
		decl := p.parseVarDecl(true, varDeclType)
		decls = append(decls, *decl)
		p.consumeOptionalToken(token.TokenTypeComma)
	}

	p.consumeSemicolon()

	if len(decls) > 0 {
		span = &token.Span{Start: varDeclTypeToken.Span.Start, End: decls[len(decls)-1].GetSpan().End}
	} else {
		span = &token.Span{Start: varDeclTypeToken.Span.Start, End: varDeclTypeToken.Span.End}
	}

	return &ast.VarDeclStmtNode{Decls: decls, Span: span}
}

func (p *Parser) parseVarDecl(allowInitializer bool, declType ast.VarDeclType) *ast.VarDeclNode {
	var initializer ast.ExprNode

	identifier := p.consumeIdentifier()
	// parse the datatype
	p.consumeToken(token.TokenTypeColon)
	dataType := p.consumeDataType()
	
	// check if the declaration has an initializer
	if allowInitializer && p.peek().Type == token.TokenTypeEqual {
		p.consume()
		initializer = p.ParseExpr()
	}

	return &ast.VarDeclNode{DeclType: declType, Identifier: identifier, Initializer: initializer, DataType: dataType}
}

func (p *Parser) ParseStmt() ast.StmtNode {
	switch p.peek().Type {
	case token.TokenTypeLet:
		return p.parseVarDeclStmt()
	case token.TokenTypeConst:
		return p.parseVarDeclStmt()
	case token.TokenTypeLeftBrace:
		return p.parseBlockStmt()
	case token.TokenTypeReturn:
		return p.parseReturnStmt()
	default:
		return p.parseExprStmt()
	}
}

func (p *Parser) GetErrors() []*error.ZeusError {
	return p.errors
}

func (p *Parser) Reset() {
	p.current = 0
	p.errors = []*error.ZeusError{}
}

func (p *Parser) ParseProgram() (*ast.ProgramNode, []*error.ZeusError) {
	p.errors = []*error.ZeusError{}
	stmts := []ast.StmtNode{}

	for !p.isEOF() {
		stmt := p.ParseStmt()
		stmts = append(stmts, stmt)
	}

	return &ast.ProgramNode{Statements: stmts}, p.errors
}

func (p *Parser) ParseExpr() ast.ExprNode {
	return p.parseExprOfPrecedence(0)
}

func (p *Parser) parseExprOfPrecedence(precedence int) ast.ExprNode {
	var left ast.ExprNode

	if p.isEOF() {
		p.expectedError("expression")
		return left
	}

	token := p.consume()
	prefixParselet := p.prefixParselets[token.Type]

	if prefixParselet == nil {
		p.expectedButGotError("expression", token)
		return left
	}

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
