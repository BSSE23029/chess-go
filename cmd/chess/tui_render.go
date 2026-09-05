package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"chess-go"
	"golang.org/x/term"
)

const (
	// The renderer owns its surface colors. Relying on the terminal profile
	// makes the board unreadable in light profiles and leaves white separator
	// gaps between colored cells.
	tuiSurface     = "\x1b[48;5;235m\x1b[38;5;252m"
	tuiFrameStart  = tuiSurface + "\x1b[H\x1b[2J"
	tuiReset       = "\x1b[0m" + tuiSurface
	tuiBold        = "\x1b[1m"
	tuiDim         = "\x1b[2m"
	tuiTitle       = "\x1b[1;38;5;75m"
	tuiAccent      = "\x1b[38;5;81m"
	tuiLightSquare = "\x1b[48;5;252m"
	tuiDarkSquare  = "\x1b[48;5;67m"
	tuiWhitePiece  = "\x1b[38;5;231m"
	tuiBlackPiece  = "\x1b[38;5;16m"
)

type tuiCache struct {
	valid        bool
	game         *chess.Game
	positionHash uint64
	theme        string
	moveCount    int
	moves        []chess.Move
	legalMoves   []chess.Move
	san          []string
	result       string
	inCheck      bool
	checkSquare  chess.Square
	captured     string
}

type tuiRenderState struct {
	valid        bool
	game         *chess.Game
	positionHash uint64
	moveCount    int
	cursor       chess.Square
	selected     chess.Square
	flipped      bool
	message      string
	whiteName    string
	blackName    string
	mode         string
	showHelp     bool
	thinking     bool
	botDetail    string
	promotion    chess.PieceType
	theme        string
	clocks       string
	width        int
	height       int
	compact      bool
}

func (ui *boardUI) invalidate() {
	ui.cache.valid = false
}

func (ui *boardUI) model(game *chess.Game, position chess.Position, boardTheme theme) *tuiCache {
	hash := position.Hash()
	moveCount := game.MoveCount()
	if ui.cache.valid && ui.cache.game == game && ui.cache.positionHash == hash && ui.cache.moveCount == moveCount && ui.cache.theme == boardTheme.label() {
		return &ui.cache
	}
	moves := game.Moves()
	inCheck := position.InCheck()
	checkSquare := chess.NoSquare
	if inCheck {
		checkSquare = checkedKingSquare(position)
	}
	ui.cache = tuiCache{
		valid:        true,
		game:         game,
		positionHash: hash,
		theme:        boardTheme.label(),
		moveCount:    moveCount,
		moves:        moves,
		legalMoves:   position.LegalMoves(),
		san:          sanMoveList(moves),
		result:       game.Result(),
		inCheck:      inCheck,
		checkSquare:  checkSquare,
		captured:     capturedSummaryWithTheme(game, boardTheme),
	}
	return &ui.cache
}

func renderInteractive(output io.Writer, game *chess.Game, ui *boardUI, flipped bool, clocks string, boardTheme theme) {
	if ui == nil {
		return
	}
	position := game.Position()
	model := ui.model(game, position, boardTheme)
	width, height := terminalSize(output)
	scale, compact := boardScaleForTerminal(width, height)
	state := tuiRenderState{
		valid:        true,
		game:         game,
		positionHash: model.positionHash,
		moveCount:    model.moveCount,
		cursor:       ui.cursor,
		selected:     ui.selected,
		flipped:      flipped,
		message:      ui.message,
		whiteName:    ui.whiteName,
		blackName:    ui.blackName,
		mode:         ui.mode,
		showHelp:     ui.showHelp,
		thinking:     ui.thinking,
		botDetail:    ui.botDetail,
		promotion:    promotionType(ui),
		theme:        boardTheme.label(),
		clocks:       clocks,
		width:        width,
		height:       height,
		compact:      compact,
	}
	if ui.rendered.valid && state.sameStatic(ui.rendered) {
		return
	}
	renderFullInteractive(output, game, ui, model, flipped, clocks, boardTheme, scale, compact, width, height)
	ui.rendered = state
}

func (s tuiRenderState) sameStatic(other tuiRenderState) bool {
	return s.game == other.game && s.positionHash == other.positionHash && s.moveCount == other.moveCount && s.cursor == other.cursor && s.selected == other.selected && s.flipped == other.flipped && s.message == other.message && s.whiteName == other.whiteName && s.blackName == other.blackName && s.mode == other.mode && s.showHelp == other.showHelp && s.thinking == other.thinking && s.botDetail == other.botDetail && s.promotion == other.promotion && s.theme == other.theme && s.clocks == other.clocks && s.width == other.width && s.height == other.height && s.compact == other.compact
}

func writeFrame(output io.Writer, frame string) {
	if _, terminal := output.(*os.File); terminal {
		if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
			frame = stripSGR(frame)
		}
	}
	writer := bufio.NewWriter(output)
	_, _ = writer.WriteString(frame)
	_ = writer.Flush()
}

func stripSGR(value string) string {
	var clean strings.Builder
	for index := 0; index < len(value); {
		if value[index] == 0x1b && index+1 < len(value) && value[index+1] == '[' {
			end := strings.IndexByte(value[index+2:], 'm')
			if end >= 0 {
				index += end + 3
				continue
			}
		}
		clean.WriteByte(value[index])
		index++
	}
	return clean.String()
}

func boardPadding(line string) string {
	if strings.HasPrefix(line, "  +") || strings.HasPrefix(line, "  ┌") || strings.HasPrefix(line, "  └") {
		return ""
	}
	return "  "
}

func terminalSize(output io.Writer) (int, int) {
	file, ok := output.(*os.File)
	if !ok {
		return 100, 40
	}
	width, height, err := term.GetSize(int(file.Fd()))
	if err != nil || width < 1 || height < 1 {
		return 100, 40
	}
	return width, height
}

func renderFullInteractive(output io.Writer, game *chess.Game, ui *boardUI, model *tuiCache, flipped bool, clocks string, boardTheme theme, scale boardScale, compact bool, width, height int) {
	position := game.Position()
	files, ranks := boardOrientation(flipped)
	legal := legalDestinations(model.legalMoves, ui.selected)
	last := lastMoveSquares(model.moves)
	separator := " · "
	if boardTheme.label() == "ascii" {
		separator = " | "
	}

	// Full-screen redraw keeps the board stable and prevents scrollback from becoming a second UI.
	var frame strings.Builder
	fmt.Fprint(&frame, tuiFrameStart)
	fmt.Fprintf(&frame, "%s%s  CHESS-GO%s  %s%s\n", tuiTitle, tuiBold, tuiReset, tuiDim, tuiReset)
	mode := tuiText(ui.mode, 24)
	if mode == "" {
		mode = "LOCAL MATCH"
	}
	fmt.Fprintf(&frame, "%s  %s%s%s theme%s %d move%s%s\n\n", tuiDim, mode, separator, strings.ToUpper(boardTheme.label()), separator, model.moveCount, plural(model.moveCount), tuiReset)

	board := boardLines(position, files, ranks, ui, legal, last, model.checkSquare, boardTheme, scale)
	rail := sidebarLines(position, ui, clocks, model, boardTheme)
	if compact {
		for _, line := range board {
			fmt.Fprintf(&frame, "  %s\n", line)
		}
	} else {
		for index, line := range board {
			fmt.Fprintf(&frame, "  %s%s  %s\n", line, boardPadding(line), railLine(rail, index, scale.cellHeight, len(board)))
		}
	}
	fmt.Fprintf(&frame, "  %s%s\n", coordinateLine(files, scale.cellWidth), tuiReset)
	legend := "reverse cursor · cyan selected · green legal · yellow last move · red check"
	if boardTheme.label() == "ascii" {
		legend = "reverse cursor | cyan selected | green legal | yellow last move | red check"
	}
	legendLabel := "LEGEND  "
	if compact {
		legend = "cursor | selected | legal | last | check"
		if width < 45 {
			legend = "cursor/legal/last/check"
			legendLabel = ""
		}
	}
	fmt.Fprintf(&frame, "    %s%s%s%s  %s\n", tuiAccent, tuiBold, legendLabel, tuiReset, legend)
	if compact {
		fmt.Fprintln(&frame)
		for _, line := range rail {
			fmt.Fprintf(&frame, "  %s\n", line)
		}
	}

	if !compact || height >= 36 {
		fmt.Fprintf(&frame, "\n%s%s  RECENT MOVES%s\n", tuiAccent, tuiBold, tuiReset)
		for _, line := range recentMoveLines(model.moves, model.san, boardTheme) {
			fmt.Fprintf(&frame, "  %s\n", line)
		}
	}
	if ui.showHelp {
		fmt.Fprintf(&frame, "\n%s%s  KEYBOARD GUIDE%s\n", tuiAccent, tuiBold, tuiReset)
		fmt.Fprintln(&frame, "  arrows / h j k l  move cursor     enter / space  select or play")
		fmt.Fprintln(&frame, "  esc  clear selection               u / r  undo or redo")
		fmt.Fprintln(&frame, "  n  new game    :  command line     q / ctrl-c  quit")
	} else {
		controls := "arrows/hjkl move" + separator + "Enter select" + separator + "u/r undo/redo" + separator + "n new" + separator + ": command" + separator + "? guide" + separator + "q quit"
		if compact {
			controls = "h/j/k/l move | Enter | u/r | n | : | ? | q"
			if width < 45 {
				controls = "h/j/k/l | Enter | ? | q"
			}
		}
		fmt.Fprintf(&frame, "\n%s  %s%s\n", tuiDim, controls, tuiReset)
	}
	if ui.message != "" {
		fmt.Fprintf(&frame, "%s%s  %s%s\n", tuiBold, tuiAccent, tuiDisplayText(ui.message, boardTheme), tuiReset)
	}
	// Raw terminal mode does not translate LF into CRLF. Prefixing each line
	// with CR keeps narrow and wide terminals aligned instead of drifting right.
	writeFrame(output, strings.ReplaceAll(formatInteractiveFrame(frame.String(), width, height), "\n", "\r\n"))
}
