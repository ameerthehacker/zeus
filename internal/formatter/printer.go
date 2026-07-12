package formatter

import (
	"strings"
	"unicode/utf8"
)

type printMode int

const (
	modeBreak printMode = iota
	modeFlat
)

// cmd is a unit of work on the printer's stack: a doc to render at a given
// indentation and in a given mode.
type cmd struct {
	indent int
	mode   printMode
	doc    Doc
}

// printDoc renders a Doc tree to a string using the Wadler/Prettier algorithm.
// It maintains a stack of pending commands (processed LIFO) and, for each group,
// measures with fits() whether the group's flat rendering stays within the print
// width; if not, the group is rendered in break mode.
func printDoc(root Doc, opts Options) string {
	var sb strings.Builder
	pos := 0 // current column

	stack := []cmd{{indent: 0, mode: modeBreak, doc: root}}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch c.doc.kind {
		case docText:
			sb.WriteString(c.doc.text)
			pos += utf8.RuneCountInString(c.doc.text)

		case docConcat:
			for i := len(c.doc.children) - 1; i >= 0; i-- {
				stack = append(stack, cmd{c.indent, c.mode, c.doc.children[i]})
			}

		case docIndent:
			stack = append(stack, cmd{c.indent + opts.IndentWidth, c.mode, c.doc.children[0]})

		case docGroup:
			content := c.doc.children[0]
			flat := cmd{c.indent, modeFlat, content}
			if !mustBreak(content) && fits(opts.PrintWidth-pos, flat, stack, opts) {
				stack = append(stack, flat)
			} else {
				stack = append(stack, cmd{c.indent, modeBreak, content})
			}

		case docLine:
			if c.mode == modeFlat {
				sb.WriteByte(' ')
				pos++
			} else {
				sb.WriteByte('\n')
				sb.WriteString(indentString(c.indent, opts))
				pos = c.indent
			}

		case docSoftline:
			if c.mode != modeFlat {
				sb.WriteByte('\n')
				sb.WriteString(indentString(c.indent, opts))
				pos = c.indent
			}

		case docHardline:
			sb.WriteByte('\n')
			sb.WriteString(indentString(c.indent, opts))
			pos = c.indent

		case docLiteralLine:
			// A newline with no indentation, so verbatim interior text keeps its
			// original columns.
			sb.WriteByte('\n')
			pos = 0

		case docIfBreak:
			if c.mode == modeBreak {
				stack = append(stack, cmd{c.indent, c.mode, c.doc.children[0]})
			} else {
				stack = append(stack, cmd{c.indent, c.mode, c.doc.children[1]})
			}
		}
	}

	return trimTrailingWhitespace(sb.String())
}

// trimTrailingWhitespace strips trailing spaces/tabs from every line so that
// blank lines (emitted as an indented newline) contain no whitespace.
func trimTrailingWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// fits reports whether `next`, followed by the remaining commands `rest`, fits
// within `remaining` columns before the next line break. Groups nested inside
// are measured optimistically in flat mode; a hardline or a break-mode line ends
// the current line (so everything before it fit).
func fits(remaining int, next cmd, rest []cmd, opts Options) bool {
	if remaining < 0 {
		return false
	}

	stack := []cmd{next}
	restIdx := len(rest) - 1

	for remaining >= 0 {
		if len(stack) == 0 {
			if restIdx < 0 {
				return true
			}
			stack = append(stack, rest[restIdx])
			restIdx--
			continue
		}

		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch c.doc.kind {
		case docText:
			remaining -= utf8.RuneCountInString(c.doc.text)

		case docConcat:
			for i := len(c.doc.children) - 1; i >= 0; i-- {
				stack = append(stack, cmd{c.indent, c.mode, c.doc.children[i]})
			}

		case docIndent:
			stack = append(stack, cmd{c.indent + opts.IndentWidth, c.mode, c.doc.children[0]})

		case docGroup:
			stack = append(stack, cmd{c.indent, modeFlat, c.doc.children[0]})

		case docLine:
			if c.mode == modeFlat {
				remaining--
			} else {
				return true
			}

		case docSoftline:
			if c.mode != modeFlat {
				return true
			}

		case docHardline, docLiteralLine:
			return true

		case docIfBreak:
			if c.mode == modeBreak {
				stack = append(stack, cmd{c.indent, c.mode, c.doc.children[0]})
			} else {
				stack = append(stack, cmd{c.indent, c.mode, c.doc.children[1]})
			}
		}
	}

	return false
}

func indentString(width int, opts Options) string {
	if opts.UseTabs {
		levels := 0
		if opts.IndentWidth > 0 {
			levels = width / opts.IndentWidth
		}
		return strings.Repeat("\t", levels)
	}
	return strings.Repeat(" ", width)
}
