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
	if boardTheme.label() != "unicode" || piece.IsEmpty() || cellWidth < 10 || cellHeight < 4 {
		return false
	}
	style := unicodePieceStyle()
	return style == "auto" || style == "sprite"
}

func pieceSpriteRow(piece chess.Piece, cellWidth, cellRow int) string {
	bitmap := pieceSpriteBitmap[piece.Type]
	if len(bitmap) == 0 || cellRow < 0 || cellRow >= 4 {
		return ""
	}
	top := spritePixelRow(bitmap, cellRow*2, spriteWidth(cellWidth))
	bottom := spritePixelRow(bitmap, cellRow*2+1, spriteWidth(cellWidth))
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
	if pixelRow == 0 || pixelRow >= 7 {
		return strings.Repeat(" ", width)
	}
	// Keep one half-cell of breathing room above and below the six-pixel
	// silhouette. The mapping preserves the sprite's base instead of simply
	// cropping its last bitmap row.
	sourceRow := (pixelRow - 1) * (len(bitmap) - 1) / 5
	return scaleSpriteRow(bitmap[sourceRow], width)
}

func spriteWidth(cellWidth int) int {
	width := cellWidth - 4
	if width < 8 {
		return 8
	}
	if width > 15 {
		return 15
	}
	return width
}

func scaleSpriteRow(row string, width int) string {
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
