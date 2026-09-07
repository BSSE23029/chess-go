package main

import (
	"strings"

	"chess-go"
)

// Piece sprites use terminal block characters instead of relying on a font's
// chess-glyph size. A sprite is eight bitmap rows tall; each terminal row
// carries two bitmap rows with upper/lower half blocks. The silhouettes are
// deliberately shaped like the six chess pieces rather than generic bars, so
// the scalable fallback remains recognizable in a real terminal.
var pieceSpriteBitmap = map[chess.PieceType][]string{
	chess.Pawn: {
		"    #    ", "   ###   ", "   ###   ", "    #    ",
		"   ###   ", "  #####  ", "  #####  ", " ####### ",
	},
	chess.Knight: {
		"    ##   ", "   ###   ", "  ####   ", "  #######",
		"  #####  ", " ######  ", " ####### ", "  #####  ",
	},
	chess.Bishop: {
		"    #    ", "   ###   ", "   # #   ", "  #####  ",
		"   ###   ", "  #####  ", "  ###### ", " ####### ",
	},
	chess.Rook: {
		"  # # #  ", "  ###### ", "    #    ", "   ###   ",
		"   ###   ", "  #####  ", "  ###### ", " ####### ",
	},
	chess.Queen: {
		"  # # #  ", " #  #  # ", "  #####  ", " ####### ",
		"   ###   ", "  #####  ", "  ###### ", " ####### ",
	},
	chess.King: {
		"    #    ", "   ###   ", "  #####  ", "    #    ",
		"   ###   ", "  #####  ", "  ###### ", " ####### ",
	},
}

func pieceSpriteEnabled(piece chess.Piece, boardTheme theme, cellWidth, cellHeight int) bool {
	if boardTheme.label() != "unicode" || piece.IsEmpty() || cellWidth < 5 || cellHeight < 2 {
		return false
	}
	style := unicodePieceStyle()
	// Keep auto on the readable Unicode glyphs. Half-block sprites can occupy
	// more cells, but their appearance varies with font rasterization and they
	// do not look like chess pieces in every terminal. Users can still opt into
	// the experimental pixel treatment explicitly with CHESS_PIECE_STYLE=sprite.
	return style == "sprite"
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
	// Resampling a narrow bitmap can otherwise leave one extra column on the
	// right (for example on the knight's head). Center the final terminal row,
	// not just the source bitmap, so every visible half-block stays aligned to
	// the cell center at every supported width.
	return centerRenderedSpriteRow(rendered.String())
}

func spritePixelRow(bitmap []string, pixelRow, width int) string {
	return spritePixelRowScaled(bitmap, pixelRow, 8, width)
}

func spritePixelRowScaled(bitmap []string, pixelRow, pixelRows, width int) string {
	if len(bitmap) == 0 || pixelRows < 1 || pixelRow < 0 || pixelRow >= pixelRows {
		return strings.Repeat(" ", maxInt(width, 0))
	}
	// Small and medium cells need every available bitmap row. Keeping the
	// silhouette's full vertical detail is more useful than padding it with
	// blank rows, which otherwise turns a rook or king into stacked bars.
	if pixelRows <= 8 {
		sourceRow := pixelRow * (len(bitmap) - 1) / maxInt(pixelRows-1, 1)
		return scaleSpriteRow(bitmap[sourceRow], width)
	}
	// Only very tall cells reserve an outer blank row to keep the silhouette
	// visually centered against the board border.
	if pixelRow == 0 || pixelRow == pixelRows-1 {
		return strings.Repeat(" ", maxInt(width, 0))
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
		start := index * len(row) / width
		end := (index + 1) * len(row) / width
		if end <= start {
			end = start + 1
		}
		occupied := false
		for _, pixel := range row[start:minInt(end, len(row))] {
			if pixel == '#' {
				occupied = true
				break
			}
		}
		if occupied {
			scaled.WriteByte('#')
		} else {
			scaled.WriteByte(' ')
		}
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

func centerRenderedSpriteRow(row string) string {
	left := strings.IndexFunc(row, func(r rune) bool { return r != ' ' })
	runes := []rune(row)
	if left < 0 {
		return row
	}
	right := len(runes) - 1
	for right >= left && runes[right] == ' ' {
		right--
	}
	if right < left {
		return row
	}
	shape := string(runes[left : right+1])
	padding := len(runes) - len([]rune(shape))
	leftPadding := padding / 2
	return strings.Repeat(" ", leftPadding) + shape + strings.Repeat(" ", padding-leftPadding)
}
