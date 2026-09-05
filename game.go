package chess

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Player chooses a move for a supplied position and supports cancellation.
type Player interface {
	ChooseMove(context.Context, Position) (Move, error)
}

// Status describes an automatically detected game state.
type Status uint8

const (
	// Ongoing means play may continue.
	Ongoing Status = iota
	// WhiteCheckmates means white won by checkmate.
	WhiteCheckmates
	// BlackCheckmates means black won by checkmate.
	BlackCheckmates
	// Stalemate is a draw because the side to move has no legal move.
	Stalemate
	// DrawFiftyMove is a draw under the automatic 50-move rule.
	DrawFiftyMove
	// DrawThreefoldRepetition is a draw by repeated position.
	DrawThreefoldRepetition
	// DrawInsufficientMaterial is a draw because mate is impossible.
	DrawInsufficientMaterial
	// DrawSeventyFiveMove is an automatic FIDE draw after 75 moves each.
	DrawSeventyFiveMove
	// DrawFivefoldRepetition is an automatic FIDE draw after five occurrences.
	DrawFivefoldRepetition
)

// RuleSet selects the draw semantics used by a game operation.
type RuleSet uint8

const (
	// CasualRules preserves the historical automatic 50-move and threefold
	// behavior used by Status and Play.
	CasualRules RuleSet = iota
	// FIDERules makes 50-move and threefold outcomes claimable, while
	// 75-move and fivefold outcomes are automatic.
	FIDERules
)

// Game tracks a main line, navigation cursor, result, and PGN metadata.
type Game struct {
	positions []Position
	moves     []Move
	cursor    int
	result    string
	tags      []PGNTag
}

// NewGame returns a game in the standard starting position.
func NewGame() *Game { return NewGameFromPosition(NewPosition()) }

// NewGameFromPosition returns a game starting from position.
func NewGameFromPosition(position Position) *Game {
	return &Game{positions: []Position{position}}
}

// Position returns the position at the current navigation cursor.
func (g *Game) Position() Position { return g.positions[g.cursor] }

// Moves returns a copy of moves through the current navigation cursor.
func (g *Game) Moves() []Move {
	moves := make([]Move, g.cursor)
	copy(moves, g.moves[:g.cursor])
	return moves
}

// MoveCount returns the number of moves through the current navigation cursor.
func (g *Game) MoveCount() int { return g.cursor }

// Captured returns captured pieces through the current navigation cursor.
func (g *Game) Captured() []Piece {
	var captured []Piece
	for index, move := range g.moves[:g.cursor] {
		if move.Flags&Capture == 0 {
			continue
		}
		square := move.To
		if move.Flags&EnPassant != 0 {
			if g.positions[index].Turn() == White {
				square -= 8
			} else {
				square += 8
			}
		}
		captured = append(captured, g.positions[index].PieceAt(square))
	}
	return captured
}

// Play validates and appends move, discarding any redo branch.
func (g *Game) Play(move Move) error {
	return g.playWithRules(move, CasualRules)
}

// PlayFIDE validates and appends a move using tournament-accurate draw rules.
// Claimable draws do not stop play until ClaimDraw is called.
func (g *Game) PlayFIDE(move Move) error {
	return g.playWithRules(move, FIDERules)
}

func (g *Game) playWithRules(move Move, rules RuleSet) error {
	if g.result != "" || g.StatusWithRules(rules) != Ongoing {
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

// PlayUCI parses and plays a move in UCI notation.
func (g *Game) PlayUCI(value string) error {
	move, err := ParseUCI(value)
	if err != nil {
		return err
	}
	return g.Play(move)
}

// PlayUCIFIDE parses and plays a move using FIDE draw semantics.
func (g *Game) PlayUCIFIDE(value string) error {
	move, err := ParseUCI(value)
	if err != nil {
		return err
	}
	return g.PlayFIDE(move)
}

// CanUndo reports whether Undo can move backward.
func (g *Game) CanUndo() bool { return g.cursor > 0 }

// CanRedo reports whether Redo can move forward.
func (g *Game) CanRedo() bool { return g.cursor < len(g.moves) }

// Undo moves the cursor backward one ply without deleting history.
func (g *Game) Undo() bool {
	if !g.CanUndo() {
		return false
	}
	g.cursor--
	g.result = ""
	return true
}

// Redo moves the cursor forward one ply when retained history exists.
func (g *Game) Redo() bool {
	if !g.CanRedo() {
		return false
	}
	g.cursor++
	return true
}

// Status returns the current automatic game status.
func (g *Game) Status() Status {
	return g.StatusWithRules(CasualRules)
}

// StatusWithRules returns the automatic status under rules. Checkmate and
// stalemate are evaluated before automatic draw thresholds as required by FIDE.
func (g *Game) StatusWithRules(rules RuleSet) Status {
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
	if position.insufficientMaterial() {
		return DrawInsufficientMaterial
	}
	if rules == FIDERules {
		if position.halfmoveClock >= 150 {
			return DrawSeventyFiveMove
		}
		if g.repetitions(position) >= 5 {
			return DrawFivefoldRepetition
		}
		return Ongoing
	}
	if position.halfmoveClock >= 100 {
		return DrawFiftyMove
	}
	if g.repetitions(position) >= 3 {
		return DrawThreefoldRepetition
	}
	return Ongoing
}

// ClaimableDraw reports a draw a player may claim under FIDE rules at the
// current position, or Ongoing when no claim is available.
func (g *Game) ClaimableDraw() Status {
	if g.StatusWithRules(FIDERules) != Ongoing {
		return Ongoing
	}
	position := g.Position()
	if position.halfmoveClock >= 100 {
		return DrawFiftyMove
	}
	if g.repetitions(position) >= 3 {
		return DrawThreefoldRepetition
	}
	return Ongoing
}

// CanClaimDraw reports whether the current position has a FIDE draw claim.
func (g *Game) CanClaimDraw() bool { return g.ClaimableDraw() != Ongoing }

// ClaimDraw records a draw when a FIDE claim is currently available.
func (g *Game) ClaimDraw() error {
	if !g.CanClaimDraw() {
		return errors.New("no claimable draw")
	}
	g.result = "1/2-1/2"
	return nil
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

// SAN returns Standard Algebraic Notation for a legal move in p.
func (p Position) SAN(move Move) (string, error) {
	legalMoves := p.LegalMoves()
	for _, legal := range legalMoves {
		if legal.From == move.From && legal.To == move.To && legal.Promotion == move.Promotion {
			return p.sanForLegal(legal, legalMoves), nil
		}
	}
	return "", fmt.Errorf("illegal move %s", move.UCI())
}

// ParseSAN resolves Standard Algebraic Notation to a legal move in p.
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

// PlaySAN parses and plays a move in Standard Algebraic Notation.
func (g *Game) PlaySAN(value string) error {
	move, err := g.Position().ParseSAN(value)
	if err != nil {
		return err
	}
	return g.Play(move)
}

// FromSAN creates a standard game by playing each SAN move in order.
func FromSAN(values []string) (*Game, error) {
	game := NewGame()
	for _, value := range values {
		if err := game.PlaySAN(value); err != nil {
			return nil, err
		}
	}
	return game, nil
}

// Result returns the PGN result marker for the current game state.
func (g *Game) Result() string {
	return g.ResultWithRules(CasualRules)
}

// ResultFIDE returns the PGN result marker under tournament-accurate draw
// semantics. Explicit results set by SetResult or ClaimDraw take precedence.
func (g *Game) ResultFIDE() string { return g.ResultWithRules(FIDERules) }

// ResultWithRules returns the PGN result marker under rules.
func (g *Game) ResultWithRules(rules RuleSet) string {
	if g.result != "" {
		return g.result
	}
	switch g.StatusWithRules(rules) {
	case WhiteCheckmates:
		return "1-0"
	case BlackCheckmates:
		return "0-1"
	case Stalemate, DrawFiftyMove, DrawThreefoldRepetition, DrawInsufficientMaterial, DrawSeventyFiveMove, DrawFivefoldRepetition:
		return "1/2-1/2"
	default:
		return "*"
	}
}
