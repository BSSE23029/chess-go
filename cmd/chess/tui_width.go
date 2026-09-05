package main

import "unicode"

// terminalTextWidth returns the number of terminal columns occupied by text.
// Rune count is not enough for emoji presentation (a chess symbol followed by
// U+FE0F occupies two columns) or combining marks. Keeping this calculation in
// one place prevents centering and viewport clipping from disagreeing.
func terminalTextWidth(text string) int {
	width := 0
	for _, r := range text {
		width += terminalRuneWidth(r)
	}
	return width
}

func terminalRuneWidth(r rune) int {
	switch {
	case r == '\n' || r == '\r' || r == '\t':
		return 0
	case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r):
		// Variation selectors are combining marks in Unicode, but terminals
		// render the emoji selector as an additional column for our explicit
		// emoji presentation. Keep that exception aligned with pieceGlyphText.
		if r == '\ufe0f' {
			return 1
		}
		return 0
	case terminalWideRune(r):
		return 2
	default:
		return 1
	}
}

// terminalWideRune is the small, dependency-free East Asian width subset
// needed by the TUI. Chess symbols, block sprites, and ASCII remain one cell;
// CJK and the common emoji ranges occupy two cells in supported terminals.
func terminalWideRune(r rune) bool {
	return r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff))
}
