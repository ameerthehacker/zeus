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
		expr := parser.ParseExprOfPrecedence(UnaryOperatorPrecedence)
		return &ast.UnaryExprNode{Operator: token, Expr: expr}
	}

	binaryOperatorParseLet := func(parser *Parser, left ast.ExprNode, token *token.Token) ast.ExprNode {
		_, isRightAssociative := RightAssociativeOperators[token.Type]
		precedence := getPrecedence(token)

		// if the operator is right associative then we need to decrease the precedence
		if isRightAssociative {
			precedence--
		}

		right := parser.ParseExprOfPrecedence(precedence)
		return &ast.BinaryExprNode{Left: left, Right: right, Operator: token}
	}

	functionCallParseLet := func(parser *Parser, left ast.ExprNode, openParen *token.Token) ast.ExprNode {
		params := []ast.ExprNode{}
		
		for {
			right := parser.ParseExprOfPrecedence(0)
			params = append(params, right)
			if parser.peek().Type != token.TokenTypeComma {
				break
			}
			parser.consume()
		}

		closeParen := parser.consumeToken(token.TokenTypeRightParen)

		return &ast.FunctionCallExprNode{Callee: left, Params: params, Span: &token.Span{Start: left.GetSpan().Start, End: closeParen.Span.End}}
	}

	prefixParselets := map[token.TokenType]func(parser *Parser, token *token.Token) ast.ExprNode{
		token.TokenTypeNumber: func(parser *Parser, token *token.Token) ast.ExprNode {
			return &ast.NumberExprNode{Value: token}
		},
		token.TokenTypeIdentifier: func(parser *Parser, token *token.Token) ast.ExprNode {
			return &ast.IdentifierExprNode{Name: token}
		},
		token.TokenTypeLeftParen: func(parser *Parser, openParen *token.Token) ast.ExprNode {
			expr := parser.ParseExprOfPrecedence(0)
			closeParen := parser.consumeToken(token.TokenTypeRightParen)
			return &ast.GroupingExprNode{Expr: expr, Span: &token.Span{Start: openParen.Span.Start, End: closeParen.Span.End}}
		},
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

func (p *Parser) parseExprStmt() ast.ExprNode {
	expr := p.ParseExpr()
	p.consumeSemicolon()

	return expr
}

func (p *Parser) parseVarDecl() ast.StmtNode {
	declTypeToken := p.consume()
	var declType ast.VarDeclType = ast.VarDeclTypeLet
	var initializer ast.ExprNode
	var span *token.Span
	// check if the declaration is a constant
	if declTypeToken.Type == token.TokenTypeConst {
		declType = ast.VarDeclTypeConst
	}

	identifier := p.consumeToken(token.TokenTypeIdentifier)
	// parse the datatype
	p.consumeToken(token.TokenTypeColon)
	dataType := p.consumeDataType()
	
	// check if the declaration has an initializer
	if p.peek().Type == token.TokenTypeEqual {
		p.consume()
		initializer = p.ParseExpr()
	}

	if initializer != nil {
		span = &token.Span{Start: declTypeToken.Span.Start, End: initializer.GetSpan().End}
	} else {
		span = &token.Span{Start: declTypeToken.Span.Start, End: identifier.Span.End}
	}

	p.consumeSemicolon()

	return &ast.VarDeclNode{DeclType: declType, Identifier: &ast.IdentifierExprNode{
		Name: identifier,
	}, Initializer: initializer, Span: span, DataType: dataType}
}

func (p *Parser) ParseStmt() ast.StmtNode {
	var stmt ast.StmtNode

	switch p.peek().Type {
	case token.TokenTypeLet:
		fallthrough
	case token.TokenTypeConst:
		stmt = p.parseVarDecl()
	default:
		expr := p.parseExprStmt()
		stmt = &ast.ExprStmtNode{Expr: expr}
	}

	return stmt
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
	return p.ParseExprOfPrecedence(0)
}

func (p *Parser) ParseExprOfPrecedence(precedence int) ast.ExprNode {
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
