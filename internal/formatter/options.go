package formatter

// Options controls how the formatter renders source. The defaults mirror
// Prettier's common conventions (80-column print width, 2-space indent).
type Options struct {
	// PrintWidth is the column the printer tries to keep lines within. Groups
	// that would exceed it are broken across multiple lines.
	PrintWidth int
	// IndentWidth is the number of spaces per indentation level (or the number
	// of columns a tab counts for when UseTabs is true).
	IndentWidth int
	// UseTabs emits a tab per indentation level instead of spaces.
	UseTabs bool
	// Semicolons appends a terminating `;` to statements and member declarations.
	// When false, terminators are omitted wherever Zeus's automatic semicolon
	// insertion makes them redundant (statements ending in a value token).
	Semicolons bool
}

// DefaultOptions returns the standard formatting options.
func DefaultOptions() Options {
	return Options{
		PrintWidth:  80,
		IndentWidth: 4,
		UseTabs:     false,
		Semicolons:  false,
	}
}
