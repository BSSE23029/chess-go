package main

import (
	"strings"

	"chess-go"
)

// Piece sprites use terminal block characters instead of relying on a font's
// chess-glyph size. A sprite is eight bitmap rows tall; each terminal row
// carries two bitmap rows with upper/lower half blocks. This keeps every cell
// aligned while making pieces visibly scale with the board.
var pieceSpriteBitmap = map[chess.PieceType][]string{
	chess.Pawn: {
		"    ###    ", "   #####   ", "   #####   ", "    ###    ",
		"    ###    ", "   #####   ", "  #######  ", "###########",
	},
	chess.Knight: {
		"    ####   ", "   #####   ", "   ##      ", "   ######  ",
		"    #####  ", "   ####    ", "  #######  ", "###########",
	},
	chess.Bishop: {
		"    ##     ", "   ####    ", "    ##     ", "   ####    ",
		"  ######   ", "   ####    ", "  #######  ", "###########",
	},
	chess.Rook: {
		"# ## ## ## ", "###########", "   #####   ", "   #####   ",
		"  #######  ", "  #######  ", " ######### ", "###########",
	},
	chess.Queen: {
		"#  ###  #  ", " ### ###   ", "  #####    ", "   ###     ",
		"  #####    ", " #######   ", "#########  ", "###########",
	},
	chess.King: {
		"    ###    ", "  #######  ", "    ###    ", "    ###    ",
		"   #####   ", "  #######  ", " ######### ", "###########",
	},
}

func pieceSpriteEnabled(piece chess.Piece, boardTheme theme, cellWidth, cellHeight int) bool {
	if boardTheme.label() != "unicode" || piece.IsEmpty() || cellWidth < 5 || cellHeight < 2 {
		return false
	}
	style := unicodePieceStyle()
	return style == "auto" || style == "sprite"
}

func pieceSpriteRow(piece chess.Piece, cellWidth, cellRow int) string {
	return pieceSpriteRowScaled(piece, cellWidth, 4, cellRow)
}

func pieceSpriteRowScaled(piece chess.Piece, cellWidth, cellHeight, cellRow int) string {
	bitmap := pieceSpriteBitmap[piece.Type]
	if len(bitmap) == 0 || cellRow < 0 || cellRow >= cellHeight {
		return ""
	}
	pixelRows := cellHeight * 2
	top := spritePixelRowScaled(bitmap, cellRow*2, pixelRows, spriteWidth(cellWidth))
	bottom := spritePixelRowScaled(bitmap, cellRow*2+1, pixelRows, spriteWidth(cellWidth))
	var rendered strings.Builder
	for index := range top {
		topOn := top[index] == '#'
		bottomOn := bottom[index] == '#'
		switch {
		case topOn && bottomOn:
			rendered.WriteRune('█')
		case topOn:
			rendered.WriteRune('▀')
		case bottomOn:
			rendered.WriteRune('▄')
		default:
			rendered.WriteByte(' ')
		}
	}
	return rendered.String()
}

func spritePixelRow(bitmap []string, pixelRow, width int) string {
	return spritePixelRowScaled(bitmap, pixelRow, 8, width)
}

func spritePixelRowScaled(bitmap []string, pixelRow, pixelRows, width int) string {
	if len(bitmap) == 0 || pixelRows < 1 || pixelRow < 0 || pixelRow >= pixelRows {
		return strings.Repeat(" ", maxInt(width, 0))
	}
	// Reserve the first and last half-row for breathing room, then map the
	// silhouette onto the rows in between. This preserves the visual baseline
	// even when a compact two-row cell has to compress the icon.
	if pixelRows > 2 && (pixelRow == 0 || pixelRow == pixelRows-1) {
		return strings.Repeat(" ", maxInt(width, 0))
	}
	if pixelRows <= 2 {
		sourceRow := pixelRow * (len(bitmap) - 1) / maxInt(pixelRows-1, 1)
		return scaleSpriteRow(bitmap[sourceRow], width)
	}
	interiorRows := maxInt(pixelRows-2, 1)
	interiorRow := pixelRow - 1
	sourceRow := interiorRow * (len(bitmap) - 1) / maxInt(interiorRows-1, 1)
	return scaleSpriteRow(bitmap[sourceRow], width)
}

func spriteWidth(cellWidth int) int {
	if cellWidth < 1 {
		return 1
	}
	if cellWidth > 15 {
		return 15
	}
	return cellWidth
}

func scaleSpriteRow(row string, width int) string {
	row = centerSpriteRow(row)
	if width <= 0 || len(row) == width {
		return row
	}
	var scaled strings.Builder
	for index := 0; index < width; index++ {
		source := index * len(row) / width
		scaled.WriteByte(row[source])
	}
	return scaled.String()
}

func centerSpriteRow(row string) string {
	left := strings.IndexByte(row, '#')
	right := strings.LastIndexByte(row, '#')
	if left < 0 || right < left {
		return row
	}
	shape := row[left : right+1]
	padding := len(row) - len(shape)
	leftPadding := padding / 2
	return strings.Repeat(" ", leftPadding) + shape + strings.Repeat(" ", padding-leftPadding)
}
