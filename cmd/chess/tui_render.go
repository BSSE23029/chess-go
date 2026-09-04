package main

import (
	"fmt"
	"io"
	"strings"

	"chess-go"
)

const (
	tuiReset       = "\x1b[0m"
	tuiBold        = "\x1b[1m"
	tuiDim         = "\x1b[2m"
	tuiTitle       = "\x1b[1;38;5;75m"
	tuiAccent      = "\x1b[38;5;81m"
	tuiLightSquare = "\x1b[48;5;252m"
	tuiDarkSquare  = "\x1b[48;5;67m"
	tuiWhitePiece  = "\x1b[38;5;231m"
	tuiBlackPiece  = "\x1b[38;5;16m"
)

func renderInteractive(output io.Writer, game *chess.Game, ui boardUI, flipped bool, clocks string, boardTheme theme) {
	position := game.Position()
	files, ranks := boardOrientation(flipped)
	legal := legalDestinations(position, ui.selected)
	last := lastMoveSquares(game)
	checkSquare := checkedKing(position)

	// Full-screen redraw keeps the board stable and prevents scrollback from becoming a second UI.
	fmt.Fprint(output, "\x1b[H\x1b[2J")
	fmt.Fprintf(output, "%s%s  CHESS-GO%s  %s%s\n", tuiTitle, tuiBold, tuiReset, tuiDim, tuiReset)
	fmt.Fprintf(output, "%s  LOCAL MATCH  ·  %s theme  ·  %d move%s%s\n\n", tuiDim, strings.ToUpper(boardTheme.label()), len(game.Moves()), plural(len(game.Moves())), tuiReset)

	board := boardLines(position, files, ranks, ui, legal, last, checkSquare, boardTheme)
	rail := sidebarLines(game, position, ui, clocks, boardTheme)
	for index, line := range board {
		fmt.Fprintf(output, "  %-38s  %s\n", line, rail[index])
	}
	fmt.Fprintf(output, "    %s%s\n", coordinateLine(files), tuiReset)
	fmt.Fprintf(output, "    %s%sLEGEND%s  reverse cursor · cyan selected · green legal · yellow last move · red check\n", tuiAccent, tuiBold, tuiReset)

	fmt.Fprintf(output, "\n%s%s  RECENT MOVES%s\n", tuiAccent, tuiBold, tuiReset)
	for _, line := range recentMoveLines(game) {
		fmt.Fprintf(output, "  %s\n", line)
	}
	if ui.showHelp {
		fmt.Fprintf(output, "\n%s%s  KEYBOARD GUIDE%s\n", tuiAccent, tuiBold, tuiReset)
		fmt.Fprintln(output, "  arrows / h j k l  move cursor     enter / space  select or play")
		fmt.Fprintln(output, "  esc  clear selection               u / r  undo or redo")
		fmt.Fprintln(output, "  n  new game    :  command line     q / ctrl-c  quit")
	} else {
		fmt.Fprintf(output, "\n%s  arrows/hjkl move · Enter select · u/r undo/redo · n new · : command · ? guide · q quit%s\n", tuiDim, tuiReset)
	}
	if ui.message != "" {
		fmt.Fprintf(output, "%s%s  %s%s\n", tuiBold, tuiAccent, ui.message, tuiReset)
	}
}

func boardOrientation(flipped bool) ([]int, []int) {
	files := []int{0, 1, 2, 3, 4, 5, 6, 7}
	ranks := []int{7, 6, 5, 4, 3, 2, 1, 0}
	if flipped {
		files = []int{7, 6, 5, 4, 3, 2, 1, 0}
		ranks = []int{0, 1, 2, 3, 4, 5, 6, 7}
	}
	return files, ranks
}

func legalDestinations(position chess.Position, selected chess.Square) map[chess.Square]bool {
	legal := make(map[chess.Square]bool)
	if selected == chess.NoSquare {
		return legal
	}
	for _, move := range position.LegalMoves() {
		if move.From == selected {
			legal[move.To] = true
		}
	}
	return legal
}

func lastMoveSquares(game *chess.Game) map[chess.Square]bool {
	last := make(map[chess.Square]bool)
	moves := game.Moves()
	if len(moves) > 0 {
		last[moves[len(moves)-1].From], last[moves[len(moves)-1].To] = true, true
	}
	return last
}

func checkedKing(position chess.Position) chess.Square {
	if !position.InCheck() {
		return chess.NoSquare
	}
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.Type == chess.King && piece.Color == position.Turn() {
			return square
		}
	}
	return chess.NoSquare
}

func boardLines(position chess.Position, files, ranks []int, ui boardUI, legal, last map[chess.Square]bool, checkSquare chess.Square, boardTheme theme) []string {
	lines := []string{"    ┌" + strings.Repeat("────┬", 7) + "────┐"}
	for _, rank := range ranks {
		var row strings.Builder
		fmt.Fprintf(&row, "%d │", rank+1)
		for index, file := range files {
			square := chess.Square(rank*8 + file)
			row.WriteString(boardCell(position.PieceAt(square), square, file+rank, ui, legal, last, checkSquare, boardTheme))
			if index < 7 {
				row.WriteRune('│')
			}
		}
		row.WriteString("│")
		lines = append(lines, row.String())
	}
	lines = append(lines, "    └"+strings.Repeat("────┴", 7)+"────┘")
	return lines
}

func boardCell(piece chess.Piece, square chess.Square, index int, ui boardUI, legal, last map[chess.Square]bool, checkSquare chess.Square, boardTheme theme) string {
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
	return fmt.Sprintf("%s%s%s %c  %s", background, state, foreground, boardTheme.glyph(piece), tuiReset)
}

func coordinateLine(files []int) string {
	var line strings.Builder
	line.WriteString("  ")
	for _, file := range files {
		fmt.Fprintf(&line, "%c  ", 'a'+file)
	}
	return line.String()
}

func sidebarLines(game *chess.Game, position chess.Position, ui boardUI, clocks string, boardTheme theme) []string {
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
	status := "Ready"
	if ui.thinking {
		status = "Bot is thinking…"
	} else if position.InCheck() {
		status = "Check — respond now"
	} else if game.Result() != "*" {
		status = "Game over: " + game.Result()
	} else if ui.message != "" {
		status = ui.message
	}
	status = tuiText(status, 34)
	captured := capturedSummaryWithTheme(game, boardTheme)
	return []string{
		fmt.Sprintf("%s%sMATCH%s", tuiAccent, tuiBold, tuiReset),
		fmt.Sprintf("  %s %s", playerMarker(position.Turn() == chess.White), white),
		fmt.Sprintf("  %s %s", playerMarker(position.Turn() == chess.Black), black),
		fmt.Sprintf("%s%sSTATUS%s", tuiAccent, tuiBold, tuiReset),
		"  " + status,
		"  " + turn,
		"  " + firstSet(clocks, "Clock off"),
		fmt.Sprintf("%s%sCAPTURED%s", tuiAccent, tuiBold, tuiReset),
		"  " + captured,
		"",
	}
}

func playerMarker(active bool) string {
	if active {
		return tuiAccent + "●" + tuiReset
	}
	return tuiDim + "○" + tuiReset
}

func recentMoveLines(game *chess.Game) []string {
	moves := game.Moves()
	if len(moves) == 0 {
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
		lines = append(lines, tuiDim+"  … older moves hidden …"+tuiReset)
	}
	for index := start; index < len(moves); index += 2 {
		white := moves[index].UCI()
		black := ""
		if index+1 < len(moves) {
			black = moves[index+1].UCI()
		}
		lines = append(lines, fmt.Sprintf("  %2d. %-7s %s", index/2+1, white, black))
	}
	return lines
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
