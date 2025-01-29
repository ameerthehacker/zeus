package ast

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
)

type ProgramNode struct {
	Statements []StmtNode
}

func (p *ProgramNode) String() string {
	statements := []string{}
	for _, stmt := range p.Statements {
		statements = append(statements, stmt.String())
	}
	return fmt.Sprintf("{ type: ProgramNode, Statements: [%s] }", strings.Join(statements, ", "))
}

func (p *ProgramNode) GetSpan() *token.Span {
	if len(p.Statements) == 0 {
		return nil
	}
	startPosition := p.Statements[0].GetSpan().Start
	endPosition := p.Statements[len(p.Statements)-1].GetSpan().End
	return &token.Span{Start: startPosition, End: endPosition}
}

func (p *ProgramNode) PrettyString() string {
	statements := []string{}
	for _, stmt := range p.Statements {
		statements = append(statements, stmt.PrettyString())
	}
	return strings.Join(statements, "\n")
}
