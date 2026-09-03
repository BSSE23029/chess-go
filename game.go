package chess

import (
	"context"
	"fmt"
	"strings"
)

type Player interface {
	ChooseMove(context.Context, Position) (Move, error)
}

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
	result    string
	tags      []PGNTag
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
	if g.result != "" || g.Status() != Ongoing {
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
	g.result = ""
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

func (p Position) SAN(move Move) (string, error) {
	legalMoves := p.LegalMoves()
	for _, legal := range legalMoves {
		if legal.From == move.From && legal.To == move.To && legal.Promotion == move.Promotion {
			return p.sanForLegal(legal, legalMoves), nil
		}
	}
	return "", fmt.Errorf("illegal move %s", move.UCI())
}

func (p Position) ParseSAN(value string) (Move, error) {
	want := normalizeSAN(value)
	legalMoves := p.LegalMoves()
	for _, move := range legalMoves {
		if normalizeSAN(p.sanForLegal(move, legalMoves)) == want {
			return move, nil
		}
	}
	return Move{}, fmt.Errorf("invalid or illegal SAN move %q", value)
}

func (p Position) sanForLegal(move Move, legalMoves []Move) string {
	if move.Flags&Castle != 0 {
		if move.To%8 == 6 {
			return p.withCheckSuffix("O-O", move)
		}
		return p.withCheckSuffix("O-O-O", move)
	}
	piece := p.board[move.From]
	var san strings.Builder
	if piece.Type != Pawn {
		san.WriteByte(map[PieceType]byte{Knight: 'N', Bishop: 'B', Rook: 'R', Queen: 'Q', King: 'K'}[piece.Type])
		p.writeDisambiguation(&san, move, piece.Type, legalMoves)
	} else if move.Flags&Capture != 0 {
		san.WriteByte('a' + byte(move.From%8))
	}
	if move.Flags&Capture != 0 {
		san.WriteByte('x')
	}
	san.WriteString(move.To.String())
	if move.Promotion != NoPiece {
		san.WriteByte('=')
		san.WriteByte(map[PieceType]byte{Knight: 'N', Bishop: 'B', Rook: 'R', Queen: 'Q'}[move.Promotion])
	}
	return p.withCheckSuffix(san.String(), move)
}

func (p Position) writeDisambiguation(san *strings.Builder, move Move, pieceType PieceType, legalMoves []Move) {
	fileUnique, rankUnique, ambiguous := true, true, false
	for _, other := range legalMoves {
		if other.From == move.From || other.To != move.To || p.board[other.From].Type != pieceType {
			continue
		}
		ambiguous = true
		fileUnique = fileUnique && other.From%8 != move.From%8
		rankUnique = rankUnique && other.From/8 != move.From/8
	}
	if !ambiguous {
		return
	}
	if fileUnique {
		san.WriteByte('a' + byte(move.From%8))
	} else if rankUnique {
		san.WriteByte('1' + byte(move.From/8))
	} else {
		san.WriteString(move.From.String())
	}
}

func (p Position) withCheckSuffix(san string, move Move) string {
	next := p.applyUnchecked(move)
	if !next.inCheck(next.turn) {
		return san
	}
	if len(next.LegalMoves()) == 0 {
		return san + "#"
	}
	return san + "+"
}

func normalizeSAN(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "0", "O"))
	return strings.TrimRight(value, "!?")
}

func (g *Game) PlaySAN(value string) error {
	move, err := g.Position().ParseSAN(value)
	if err != nil {
		return err
	}
	return g.Play(move)
}

func FromSAN(values []string) (*Game, error) {
	game := NewGame()
	for _, value := range values {
		if err := game.PlaySAN(value); err != nil {
			return nil, err
		}
	}
	return game, nil
}

func (g *Game) Result() string {
	if g.result != "" {
		return g.result
	}
	switch g.Status() {
	case WhiteCheckmates:
		return "1-0"
	case BlackCheckmates:
		return "0-1"
	case Stalemate, DrawFiftyMove, DrawThreefoldRepetition, DrawInsufficientMaterial:
		return "1/2-1/2"
	default:
		return "*"
	}
}
