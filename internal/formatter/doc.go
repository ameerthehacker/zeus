package formatter

// The Doc IR is a Prettier-style intermediate representation. A formatter
// translates the AST into a tree of Docs, and printer.go renders that tree to
// text, deciding for each `group` whether it fits on one line (flat) or must
// break across lines. See wiki/formatter.md for the algorithm.

type docKind int

const (
	docText        docKind = iota // literal text (no newlines)
	docConcat                     // a sequence of child docs
	docLine                       // " " when flat, newline+indent when broken
	docSoftline                   // ""  when flat, newline+indent when broken
	docHardline                   // always newline+indent; forces enclosing groups to break
	docLiteralLine                // always a newline with NO indent; forces breaks (verbatim text)
	docGroup                      // children[0]: try flat, break if it doesn't fit
	docIndent                     // children[0]: rendered with one more indent level
	docIfBreak                    // children[0] when broken, children[1] when flat
)

// Doc is a node in the document IR. Leaf kinds (line/softline/hardline) carry no
// payload; docText carries text; the rest carry child docs.
type Doc struct {
	kind     docKind
	text     string
	children []Doc
}

var (
	// line is a space in flat mode, a newline in break mode.
	line = Doc{kind: docLine}
	// softline is nothing in flat mode, a newline in break mode.
	softline = Doc{kind: docSoftline}
	// hardline is always a newline and forces every enclosing group to break.
	hardline = Doc{kind: docHardline}
	// literalLine is a newline with no indentation, used to emit verbatim
	// multi-line text (block comments) without re-indenting the interior.
	literalLine = Doc{kind: docLiteralLine}
	// nilDoc is an empty placeholder (renders to nothing).
	nilDoc = Doc{kind: docText, text: ""}
)

func text(s string) Doc {
	return Doc{kind: docText, text: s}
}

func concat(docs ...Doc) Doc {
	return Doc{kind: docConcat, children: docs}
}

func group(d Doc) Doc {
	return Doc{kind: docGroup, children: []Doc{d}}
}

func indent(d Doc) Doc {
	return Doc{kind: docIndent, children: []Doc{d}}
}

// ifBreak renders broken when its enclosing group breaks, flat otherwise.
// It is used for trailing commas: ifBreak(text(","), nilDoc).
func ifBreak(broken, flat Doc) Doc {
	return Doc{kind: docIfBreak, children: []Doc{broken, flat}}
}

// join interleaves sep between the given docs.
func join(sep Doc, docs []Doc) Doc {
	parts := make([]Doc, 0, len(docs)*2)
	for i, d := range docs {
		if i > 0 {
			parts = append(parts, sep)
		}
		parts = append(parts, d)
	}
	return concat(parts...)
}

// mustBreak reports whether a doc contains a forced break (a hardline) anywhere
// that is not hidden behind an ifBreak. Such a doc can never render flat, so its
// enclosing groups are put straight into break mode without measuring.
func mustBreak(d Doc) bool {
	switch d.kind {
	case docHardline, docLiteralLine:
		return true
	case docIfBreak:
		// The contents are conditional on the group mode, so they do not force a break.
		return false
	case docConcat, docGroup, docIndent:
		for _, c := range d.children {
			if mustBreak(c) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
