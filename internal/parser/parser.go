package parser

import "ameerthehacker/zeus/internal/token"

type Parser struct {
	tokens []*token.Token
}

func NewParser(tokens []*token.Token) *Parser {
	return &Parser{tokens: tokens}
}
