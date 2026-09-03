package chess

import "fmt"

type Status uint8

const (
	Ongoing Status = iota
	WhiteCheckmates
	BlackCheckmates
	Stalemate
	DrawFiftyMove
	DrawThreefoldRepetition
	DrawInsufficientMaterial
)

type Game struct {
	positions []Position
	moves     []Move
	cursor    int
}

func NewGame() *Game { return NewGameFromPosition(NewPosition()) }

func NewGameFromPosition(position Position) *Game {
	return &Game{positions: []Position{position}}
}

func (g *Game) Position() Position { return g.positions[g.cursor] }

func (g *Game) Moves() []Move {
	moves := make([]Move, g.cursor)
	copy(moves, g.moves[:g.cursor])
	return moves
}

func (g *Game) Play(move Move) error {
	if g.Status() != Ongoing {
		return fmt.Errorf("game is over")
	}
	position := g.Position()
	legal, ok := position.resolveMove(move)
	if !ok {
		return fmt.Errorf("illegal move %s", move.UCI())
	}
	next := position.applyUnchecked(legal)
	g.positions = append(g.positions[:g.cursor+1], next)
	g.moves = append(g.moves[:g.cursor], legal)
	g.cursor++
	return nil
}

func (g *Game) PlayUCI(value string) error {
	move, err := ParseUCI(value)
	if err != nil {
		return err
	}
	return g.Play(move)
}

func (g *Game) CanUndo() bool { return g.cursor > 0 }
func (g *Game) CanRedo() bool { return g.cursor < len(g.moves) }

func (g *Game) Undo() bool {
	if !g.CanUndo() {
		return false
	}
	g.cursor--
	return true
}

func (g *Game) Redo() bool {
	if !g.CanRedo() {
		return false
	}
	g.cursor++
	return true
}

func (g *Game) Status() Status {
	position := g.Position()
	if len(position.LegalMoves()) == 0 {
		if position.inCheck(position.turn) {
			if position.turn == White {
				return BlackCheckmates
			}
			return WhiteCheckmates
		}
		return Stalemate
	}
	if position.halfmoveClock >= 100 {
		return DrawFiftyMove
	}
	if g.repetitions(position) >= 3 {
		return DrawThreefoldRepetition
	}
	if position.insufficientMaterial() {
		return DrawInsufficientMaterial
	}
	return Ongoing
}

func (g *Game) repetitions(position Position) int {
	count := 0
	want := position.repetitionKey()
	for i := 0; i <= g.cursor; i++ {
		if g.positions[i].repetitionKey() == want {
			count++
		}
	}
	return count
}

type positionKey struct {
	board     [64]Piece
	turn      Color
	castling  CastlingRights
	enPassant Square
}

func (p Position) repetitionKey() positionKey {
	key := positionKey{board: p.board, turn: p.turn, castling: p.castling, enPassant: p.enPassant}
	if p.enPassant != NoSquare {
		hasCapture := false
		for _, move := range p.LegalMoves() {
			hasCapture = hasCapture || move.Flags&EnPassant != 0
		}
		if !hasCapture {
			key.enPassant = NoSquare
		}
	}
	return key
}

func (p Position) insufficientMaterial() bool {
	minors, knights, bishopColor := 0, 0, -1
	for square, piece := range p.board {
		switch piece.Type {
		case Pawn, Rook, Queen:
			return false
		case Knight:
			minors++
			knights++
		case Bishop:
			minors++
			color := (square%8 + square/8) % 2
			if bishopColor >= 0 && bishopColor != color {
				return false
			}
			bishopColor = color
		}
	}
	return minors <= 1 || (knights == 0 && bishopColor >= 0)
}
