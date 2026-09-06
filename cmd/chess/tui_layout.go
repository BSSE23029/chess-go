package main

import (
	"fmt"
	"os"
	"strings"

	"chess-go"
)

func boardOrientation(flipped bool) ([]int, []int) {
	files := []int{0, 1, 2, 3, 4, 5, 6, 7}
	ranks := []int{7, 6, 5, 4, 3, 2, 1, 0}
	if flipped {
		files = []int{7, 6, 5, 4, 3, 2, 1, 0}
		ranks = []int{0, 1, 2, 3, 4, 5, 6, 7}
	}
	return files, ranks
}

func legalDestinations(moves []chess.Move, selected chess.Square) [64]bool {
	var legal [64]bool
	if selected == chess.NoSquare {
		return legal
	}
	for _, move := range moves {
		if move.From == selected {
			legal[move.To] = true
		}
	}
	return legal
}

func lastMoveSquares(moves []chess.Move) [64]bool {
	var last [64]bool
	if len(moves) > 0 {
		last[moves[len(moves)-1].From], last[moves[len(moves)-1].To] = true, true
	}
	return last
}

func checkedKingSquare(position chess.Position) chess.Square {
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.Type == chess.King && piece.Color == position.Turn() {
			return square
		}
	}
	return chess.NoSquare
}

type boardBorder struct {
	vertical    string
	horizontal  string
	leftTop     string
	topJoin     string
	rightTop    string
	leftBottom  string
	bottomJoin  string
	rightBottom string
}

type boardScale struct {
	cellWidth  int
	cellHeight int
}

func boardScaleForTerminal(width, height int) (boardScale, bool) {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}
	cellWidth := 4
	switch {
	case width >= 250:
		cellWidth = 20
	case width >= 230:
		cellWidth = 18
	case width >= 210:
		cellWidth = 18
	case width >= 180:
		cellWidth = 14
	case width >= 150:
		cellWidth = 11
	case width >= 125:
		cellWidth = 8
	case width >= 115:
		cellWidth = 6
	case width >= 95:
		cellWidth = 5
	case width <= 30:
		cellWidth = 1
	}
	boardWidth := 12 + cellWidth*8
	compact := width < boardWidth+48
	cellHeight := 1
	if !compact {
		// A one-row board leaves the Unicode pieces at the mercy of the
		// terminal font. Use two rows as soon as the normal dashboard fits,
		// then grow the board in whole rows without letting the footer fall
		// outside the viewport.
		switch {
		case height >= 58:
			cellHeight = 4
		case height >= 44:
			cellHeight = 3
		case height >= 30:
			cellHeight = 2
		}
	}
	return boardScale{cellWidth: cellWidth, cellHeight: cellHeight}, compact
}

func borderForTheme(boardTheme theme) boardBorder {
	if boardTheme.label() == "ascii" {
		return boardBorder{vertical: "|", horizontal: "-", leftTop: "+", topJoin: "+", rightTop: "+", leftBottom: "+", bottomJoin: "+", rightBottom: "+"}
	}
	return boardBorder{vertical: "│", horizontal: "─", leftTop: "┌", topJoin: "┬", rightTop: "┐", leftBottom: "└", bottomJoin: "┴", rightBottom: "┘"}
}

func boardLines(position chess.Position, files, ranks []int, ui *boardUI, legal, last [64]bool, checkSquare chess.Square, boardTheme theme, scale boardScale) []string {
	border := borderForTheme(boardTheme)
	cellWidth := maxInt(scale.cellWidth, 1)
	cellHeight := maxInt(scale.cellHeight, 1)
	lines := []string{strings.Repeat(" ", 2) + border.leftTop + strings.Repeat(strings.Repeat(border.horizontal, cellWidth)+border.topJoin, 7) + strings.Repeat(border.horizontal, cellWidth) + border.rightTop}
	for _, rank := range ranks {
		for repeat := range cellHeight {
			var row strings.Builder
			if repeat == (cellHeight-1)/2 {
				fmt.Fprintf(&row, "%d %s", rank+1, border.vertical)
			} else {
				fmt.Fprintf(&row, "  %s", border.vertical)
			}
			for index, file := range files {
				square := chess.Square(rank*8 + file)
				row.WriteString(boardCellAtRow(position.PieceAt(square), square, file+rank, ui, legal, last, checkSquare, boardTheme, cellWidth, cellHeight, repeat))
				if index < 7 {
					row.WriteString(border.vertical)
				}
			}
			row.WriteString(border.vertical)
			lines = append(lines, row.String())
		}
	}
	lines = append(lines, strings.Repeat(" ", 2)+border.leftBottom+strings.Repeat(strings.Repeat(border.horizontal, cellWidth)+border.bottomJoin, 7)+strings.Repeat(border.horizontal, cellWidth)+border.rightBottom)
	return lines
}

func boardCell(piece chess.Piece, square chess.Square, index int, ui *boardUI, legal, last [64]bool, checkSquare chess.Square, boardTheme theme, cellWidth int) string {
	return boardCellGlyph(piece, square, index, ui, legal, last, checkSquare, boardTheme, boardTheme.glyph(piece), cellWidth)
}

func boardCellAtRow(piece chess.Piece, square chess.Square, index int, ui *boardUI, legal, last [64]bool, checkSquare chess.Square, boardTheme theme, cellWidth, cellHeight, cellRow int) string {
	if pieceSpriteEnabled(piece, boardTheme, cellWidth, cellHeight) {
		return boardCellGlyphText(piece, square, index, ui, legal, last, checkSquare, boardTheme, pieceSpriteRowScaled(piece, cellWidth, cellHeight, cellRow), cellWidth)
	}
	if cellRow != (cellHeight-1)/2 {
		return boardCellGlyph(piece, square, index, ui, legal, last, checkSquare, boardTheme, ' ', cellWidth)
	}
	return boardCell(piece, square, index, ui, legal, last, checkSquare, boardTheme, cellWidth)
}

func boardCellWithoutPiece(piece chess.Piece, square chess.Square, index int, ui *boardUI, legal, last [64]bool, checkSquare chess.Square, boardTheme theme, cellWidth int) string {
	return boardCellGlyph(piece, square, index, ui, legal, last, checkSquare, boardTheme, ' ', cellWidth)
}

func boardCellGlyph(piece chess.Piece, square chess.Square, index int, ui *boardUI, legal, last [64]bool, checkSquare chess.Square, boardTheme theme, glyph rune, cellWidth int) string {
	return boardCellGlyphText(piece, square, index, ui, legal, last, checkSquare, boardTheme, string(glyph), cellWidth)
}

func boardCellGlyphText(piece chess.Piece, square chess.Square, index int, ui *boardUI, legal, last [64]bool, checkSquare chess.Square, boardTheme theme, glyphText string, cellWidth int) string {
	cellWidth = maxInt(cellWidth, 1)
	background := tuiDarkSquare
	if index%2 == 0 {
		background = tuiLightSquare
	}
	state := ""
	switch {
	case square == ui.cursor:
		state = "\x1b[7m"
	case square == checkSquare:
		state = "\x1b[41m"
	case square == ui.selected:
		state = "\x1b[46m"
	case legal[square]:
		state = "\x1b[42m"
	case last[square]:
		state = "\x1b[43m"
	}
	foreground := tuiBlackPiece
	if !piece.IsEmpty() && piece.Color == chess.White {
		foreground = tuiWhitePiece
	}
	weight := ""
	// Terminals do not expose a portable font-size control. On wider Unicode
	// cells, bold presentation makes the fixed-size chess glyphs more legible;
	// vertical centering keeps them visually scaled with the larger box.
	if boardTheme.label() == "unicode" && !piece.IsEmpty() && cellWidth >= 4 {
		weight = tuiBold
	}
	glyphWidth := terminalTextWidth(glyphText)
	if glyphWidth == 0 {
		glyphText = " "
		glyphWidth = 1
	}
	if glyphWidth == 1 {
		glyphText, glyphWidth = pieceGlyphText(piece, []rune(glyphText)[0], boardTheme, cellWidth)
	}
	if glyphWidth > cellWidth {
		glyphText = truncateTerminalText(glyphText, cellWidth)
		glyphWidth = terminalTextWidth(glyphText)
	}
	left := (cellWidth - glyphWidth) / 2
	right := cellWidth - left - glyphWidth
	return fmt.Sprintf("%s%s%s%s%s%s%s%s", background, state, foreground, weight, strings.Repeat(" ", left), glyphText, strings.Repeat(" ", right), tuiReset)
}

func pieceGlyphText(piece chess.Piece, glyph rune, boardTheme theme, cellWidth int) (string, int) {
	text := string(glyph)
	if boardTheme.label() != "unicode" || piece.IsEmpty() || glyph == ' ' || cellWidth < 10 || unicodePieceStyle() != "emoji" || !terminalSupportsEmoji() {
		return text, 1
	}
	// Emoji presentation is wider and usually has a larger terminal glyph than
	// text presentation. Reserve two columns so the board remains aligned.
	return text + "\ufe0f", 2
}

func unicodePieceStyle() string {
	style := strings.ToLower(strings.TrimSpace(os.Getenv("CHESS_PIECE_STYLE")))
	switch style {
	case "text":
		return "text"
	case "emoji":
		return "emoji"
	case "sprite", "icon", "icons":
		return "sprite"
	case "", "auto":
		return "auto"
	default:
		return "text"
	}
}

func piecePresentationLabel(boardTheme theme, scale boardScale) string {
	if boardTheme.label() != "unicode" {
		return "letters"
	}
	style := unicodePieceStyle()
	if style == "auto" {
		if pieceSpriteEnabled(chess.Piece{Color: chess.White, Type: chess.Pawn}, boardTheme, scale.cellWidth, scale.cellHeight) {
			return "icons"
		}
		return "text"
	}
	if style == "sprite" || style == "emoji" {
		if style == "sprite" || (scale.cellWidth >= 10 && terminalSupportsEmoji()) {
			return style
		}
	}
	return "text"
}

func terminalSupportsEmoji() bool {
	program := strings.ToLower(strings.TrimSpace(os.Getenv("TERM_PROGRAM")))
	if program == "iterm.app" || program == "wezterm" || program == "ghostty" {
		return true
	}
	return strings.Contains(strings.ToLower(os.Getenv("TERM")), "kitty")
}

func coordinateLine(files []int, cellWidth int) string {
	var line strings.Builder
	line.WriteString(strings.Repeat(" ", 2))
	cellWidth = maxInt(cellWidth, 1)
	for _, file := range files {
		left := (cellWidth - 1) / 2
		right := cellWidth - left - 1
		fmt.Fprintf(&line, "%s%c%s", strings.Repeat(" ", left), 'a'+file, strings.Repeat(" ", right))
	}
	return line.String()
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func sidebarLines(position chess.Position, ui *boardUI, clocks string, model *tuiCache, boardTheme theme) []string {
	white, black := ui.whiteName, ui.blackName
	if white == "" {
		white = "White"
	}
	if black == "" {
		black = "Black"
	}
	white = tuiText(white, 20)
	black = tuiText(black, 20)
	turn := colorName(position.Turn()) + " to move"
	separator := " · "
	if boardTheme.label() == "ascii" {
		separator = " | "
	}
	status := "Ready"
	if len(ui.promotion) > 0 {
		status = fmt.Sprintf("Promote to %s", pieceTypeName(ui.promotion[ui.promotionIndex].Promotion))
	} else if ui.thinking {
		status = "Bot is thinking..."
	} else if model.inCheck {
		status = "Check - respond now"
	} else if model.result != "*" {
		status = "Game over: " + model.result
	} else if ui.message != "" {
		status = ui.message
	}
	if !ui.thinking && ui.botDetail != "" && ui.message == "" {
		status = ui.botDetail
	}
	status = tuiText(status, 34)
	if boardTheme.label() != "ascii" {
		status = strings.ReplaceAll(status, "...", "…")
		status = strings.ReplaceAll(status, " - ", " — ")
	}
	captured := model.captured
	if boardTheme.label() == "ascii" {
		captured = strings.ReplaceAll(captured, " · ", " / ")
	}
	turnDetail := turn
	if ui.selected != chess.NoSquare {
		piece := position.PieceAt(ui.selected)
		legalCount := 0
		for _, move := range model.legalMoves {
			if move.From == ui.selected {
				legalCount++
			}
		}
		turnDetail = fmt.Sprintf("%s%sSelected %s on %s (%d legal)", turn, separator, pieceTypeName(piece.Type), ui.selected, legalCount)
	}
	return []string{
		fmt.Sprintf("%s%sMATCH%s", tuiAccent, tuiBold, tuiReset),
		fmt.Sprintf("  %s %s", playerMarker(position.Turn() == chess.White, boardTheme), white),
		fmt.Sprintf("  %s %s", playerMarker(position.Turn() == chess.Black, boardTheme), black),
		fmt.Sprintf("%s%sSTATUS%s", tuiAccent, tuiBold, tuiReset),
		"  " + status,
		"  " + tuiText(turnDetail, 38),
		"  " + firstSet(clocks, "Clock off"),
		fmt.Sprintf("%s%sCAPTURED%s", tuiAccent, tuiBold, tuiReset),
		"  " + captured,
		"",
	}
}

func playerMarker(active bool, boardTheme theme) string {
	marker := "●"
	if boardTheme.label() == "ascii" {
		marker = "*"
		if !active {
			marker = "-"
		}
	}
	if active {
		return tuiAccent + marker + tuiReset
	}
	if boardTheme.label() != "ascii" {
		marker = "○"
	}
	return tuiDim + marker + tuiReset
}

func promotionType(ui *boardUI) chess.PieceType {
	if ui == nil || len(ui.promotion) == 0 || ui.promotionIndex >= len(ui.promotion) {
		return chess.NoPiece
	}
	return ui.promotion[ui.promotionIndex].Promotion
}

func pieceTypeName(piece chess.PieceType) string {
	return map[chess.PieceType]string{
		chess.Pawn:   "Pawn",
		chess.Queen:  "Queen",
		chess.Rook:   "Rook",
		chess.Bishop: "Bishop",
		chess.Knight: "Knight",
		chess.King:   "King",
	}[piece]
}

func recentMoveLines(moves []chess.Move, notation []string, boardTheme theme) []string {
	if len(moves) == 0 {
		if boardTheme.label() == "ascii" {
			return []string{tuiDim + "  - no moves yet -" + tuiReset}
		}
		return []string{tuiDim + "  — no moves yet —" + tuiReset}
	}
	start := len(moves) - 8
	if start < 0 {
		start = 0
	}
	if start%2 != 0 {
		start--
	}
	lines := make([]string, 0, 5)
	if start > 0 {
		if boardTheme.label() == "ascii" {
			lines = append(lines, tuiDim+"  ... older moves hidden ..."+tuiReset)
		} else {
			lines = append(lines, tuiDim+"  … older moves hidden …"+tuiReset)
		}
	}
	for index := start; index < len(moves); index += 2 {
		white := moveLabel(moves[index], notation, index)
		black := ""
		if index+1 < len(moves) {
			black = moveLabel(moves[index+1], notation, index+1)
		}
		lines = append(lines, fmt.Sprintf("  %2d. %-7s %s", index/2+1, white, black))
	}
	return lines
}

func sanMoveList(moves []chess.Move) []string {
	replay := chess.NewGame()
	notation := make([]string, len(moves))
	for index, move := range moves {
		san, err := replay.Position().SAN(move)
		if err != nil || replay.Play(move) != nil {
			return nil
		}
		notation[index] = san
	}
	return notation
}

func moveLabel(move chess.Move, notation []string, index int) string {
	uci := move.UCI()
	if index >= len(notation) || notation[index] == "" || notation[index] == uci {
		return uci
	}
	return notation[index] + " " + tuiDim + "(" + uci + ")" + tuiReset
}

func plural(value int) string {
	if value == 1 {
		return ""
	}
	return "s"
}

func tuiText(value string, limit int) string {
	var clean strings.Builder
	for _, character := range strings.TrimSpace(value) {
		if character < 32 || character == 127 {
			continue
		}
		clean.WriteRune(character)
	}
	runes := []rune(clean.String())
	if limit <= 0 || len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit-1]) + "…"
}

func tuiDisplayText(value string, boardTheme theme) string {
	if boardTheme.label() == "ascii" {
		return strings.NewReplacer("—", "-", "…", "...", "·", "/").Replace(value)
	}
	return value
}
