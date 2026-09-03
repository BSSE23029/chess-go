// Package chess provides legal move generation, position and game state,
// FEN, SAN, PGN, draw detection, and reversible moves for chess applications.
package chess

import (
	"fmt"
	"strconv"
	"strings"
)

// Color identifies one side in a chess position.
type Color uint8

const (
	// White is the side that moves first.
	White Color = iota
	// Black is the opposing side.
	Black
)

// Opponent returns the other color.
func (c Color) Opponent() Color { return c ^ 1 }

// PieceType identifies a kind of chess piece.
type PieceType uint8

const (
	// NoPiece represents an empty square.
	NoPiece PieceType = iota
	// Pawn is a pawn.
	Pawn
	// Knight is a knight.
	Knight
	// Bishop is a bishop.
	Bishop
	// Rook is a rook.
	Rook
	// Queen is a queen.
	Queen
	// King is a king.
	King
)

// MoveFlags describe special properties of a legal move.
type MoveFlags uint8

const (
	// Capture marks a move that captures a piece.
	Capture MoveFlags = 1 << iota
	// EnPassant marks an en-passant capture.
	EnPassant
	// Castle marks a castling move.
	Castle
	// PawnDouble marks a pawn's initial two-square move.
	PawnDouble
)

// Move describes a move using source, destination, promotion, and flags.
type Move struct {
	// From is the source square.
	From Square
	// To is the destination square.
	To Square
	// Promotion is the requested promotion piece, or NoPiece.
	Promotion PieceType
	// Flags describe capture, castling, en-passant, and pawn-double moves.
	Flags MoveFlags
}

// ParseUCI parses long algebraic UCI notation such as "e2e4" or "a7a8q".
func ParseUCI(value string) (Move, error) {
	if len(value) != 4 && len(value) != 5 {
		return Move{}, fmt.Errorf("invalid UCI move %q", value)
	}
	from, err := ParseSquare(value[:2])
	if err != nil {
		return Move{}, fmt.Errorf("invalid UCI move %q", value)
	}
	to, err := ParseSquare(value[2:4])
	if err != nil {
		return Move{}, fmt.Errorf("invalid UCI move %q", value)
	}
	move := Move{From: from, To: to}
	if len(value) == 5 {
		promotions := map[byte]PieceType{'q': Queen, 'r': Rook, 'b': Bishop, 'n': Knight}
		var ok bool
		move.Promotion, ok = promotions[value[4]]
		if !ok {
			return Move{}, fmt.Errorf("invalid UCI move %q", value)
		}
	}
	return move, nil
}

// UCI returns long algebraic UCI notation for m.
func (m Move) UCI() string {
	value := m.From.String() + m.To.String()
	if m.Promotion != NoPiece {
		value += string(map[PieceType]byte{Queen: 'q', Rook: 'r', Bishop: 'b', Knight: 'n'}[m.Promotion])
	}
	return value
}

// Piece combines a piece type and color. Its zero value is an empty square.
type Piece struct {
	// Type identifies the piece or NoPiece.
	Type PieceType
	// Color identifies the owning side when Type is not NoPiece.
	Color Color
}

// IsEmpty reports whether p represents an empty square.
func (p Piece) IsEmpty() bool { return p.Type == NoPiece }

// Square identifies a board square from a1 (0) through h8 (63).
type Square uint8

// NoSquare represents the absence of a board square.
const NoSquare Square = 64

// ParseSquare parses algebraic coordinates such as "e4".
func ParseSquare(value string) (Square, error) {
	if len(value) != 2 || value[0] < 'a' || value[0] > 'h' || value[1] < '1' || value[1] > '8' {
		return NoSquare, fmt.Errorf("invalid square %q", value)
	}
	return Square((value[1]-'1')*8 + value[0] - 'a'), nil
}

// String returns algebraic coordinates, or "-" for an invalid square.
func (s Square) String() string {
	if s > 63 {
		return "-"
	}
	return string([]byte{'a' + byte(s%8), '1' + byte(s/8)})
}

// CastlingRights is a bit set of currently available castling rights.
type CastlingRights uint8

const (
	// WhiteKingSide permits white to castle king-side.
	WhiteKingSide CastlingRights = 1 << iota
	// WhiteQueenSide permits white to castle queen-side.
	WhiteQueenSide
	// BlackKingSide permits black to castle king-side.
	BlackKingSide
	// BlackQueenSide permits black to castle queen-side.
	BlackQueenSide
)

// InitialFEN is the standard chess starting position.
const InitialFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type zobristKeys struct {
	pieces    [2][7][64]uint64
	turn      uint64
	castling  [16]uint64
	enPassant [64]uint64
}

var zobrist = newZobristKeys()

func newZobristKeys() zobristKeys {
	var keys zobristKeys
	seed := uint64(0x9e3779b97f4a7c15)
	next := func() uint64 {
		seed += 0x9e3779b97f4a7c15
		value := seed
		value = (value ^ value>>30) * 0xbf58476d1ce4e5b9
		value = (value ^ value>>27) * 0x94d049bb133111eb
		return value ^ value>>31
	}
	for color := range keys.pieces {
		for piece := Pawn; piece <= King; piece++ {
			for square := range keys.pieces[color][piece] {
				keys.pieces[color][piece][square] = next()
			}
		}
	}
	keys.turn = next()
	for index := range keys.castling {
		keys.castling[index] = next()
	}
	for index := range keys.enPassant {
		keys.enPassant[index] = next()
	}
	return keys
}

// Position is an immutable-by-value chess position with cached Zobrist hash.
type Position struct {
	board          [64]Piece
	turn           Color
	castling       CastlingRights
	enPassant      Square
	halfmoveClock  uint16
	fullmoveNumber uint16
	hash           uint64
}

// NewPosition returns the standard chess starting position.
func NewPosition() Position {
	position, err := ParseFEN(InitialFEN)
	if err != nil {
		panic(err)
	}
	return position
}

// PieceAt returns the piece on square, or an empty piece for an invalid square.
func (p Position) PieceAt(square Square) Piece {
	if square > 63 {
		return Piece{}
	}
	return p.board[square]
}

// Turn returns the side to move.
func (p Position) Turn() Color { return p.turn }

// Castling returns the position's available castling rights.
func (p Position) Castling() CastlingRights { return p.castling }

// EnPassant returns the en-passant target, or NoSquare.
func (p Position) EnPassant() Square { return p.enPassant }

// HalfmoveClock returns halfmoves since the last pawn move or capture.
func (p Position) HalfmoveClock() uint16 { return p.halfmoveClock }

// FullmoveNumber returns the FEN fullmove number.
func (p Position) FullmoveNumber() uint16 { return p.fullmoveNumber }

// InCheck reports whether the side to move is in check.
func (p Position) InCheck() bool { return p.inCheck(p.turn) }

// Hash returns the incrementally maintained Zobrist key. Move clocks are
// intentionally excluded so equivalent search positions share a key.
func (p Position) Hash() uint64 { return p.hash }

func (p Position) calculateHash() uint64 {
	hash := zobrist.castling[p.castling]
	if p.turn == Black {
		hash ^= zobrist.turn
	}
	if p.enPassant != NoSquare {
		hash ^= zobrist.enPassant[p.enPassant]
	}
	for square, piece := range p.board {
		if !piece.IsEmpty() {
			hash ^= zobrist.pieces[piece.Color][piece.Type][square]
		}
	}
	return hash
}

// ParseFEN parses and semantically validates a complete six-field FEN.
func ParseFEN(fen string) (Position, error) {
	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return Position{}, fmt.Errorf("FEN must contain 6 fields")
	}

	position := Position{enPassant: NoSquare}
	if err := position.parseBoard(fields[0]); err != nil {
		return Position{}, err
	}
	switch fields[1] {
	case "w":
		position.turn = White
	case "b":
		position.turn = Black
	default:
		return Position{}, fmt.Errorf("invalid active color %q", fields[1])
	}
	if err := position.parseCastling(fields[2]); err != nil {
		return Position{}, err
	}
	if fields[3] != "-" {
		square, err := ParseSquare(fields[3])
		if err != nil || (square/8 != 2 && square/8 != 5) {
			return Position{}, fmt.Errorf("invalid en passant target %q", fields[3])
		}
		position.enPassant = square
	}
	var err error
	position.halfmoveClock, err = parseUint16(fields[4], "halfmove clock", true)
	if err != nil {
		return Position{}, err
	}
	position.fullmoveNumber, err = parseUint16(fields[5], "fullmove number", false)
	if err != nil {
		return Position{}, err
	}
	if err := position.validate(); err != nil {
		return Position{}, err
	}
	position.hash = position.calculateHash()
	return position, nil
}

func (p Position) validate() error {
	var counts [2][7]int
	kings := [2]Square{NoSquare, NoSquare}
	for square, piece := range p.board {
		if piece.IsEmpty() {
			continue
		}
		counts[piece.Color][piece.Type]++
		if piece.Type == Pawn && (square/8 == 0 || square/8 == 7) {
			return fmt.Errorf("pawn on invalid square %s", Square(square))
		}
		if piece.Type == King {
			kings[piece.Color] = Square(square)
		}
	}
	for _, color := range []Color{White, Black} {
		if counts[color][King] != 1 {
			return fmt.Errorf("%s must have exactly one king", colorName(color))
		}
		total := 0
		for _, count := range counts[color] {
			total += count
		}
		if total > 16 || counts[color][Pawn] > 8 {
			return fmt.Errorf("%s has impossible material", colorName(color))
		}
		extra := max(counts[color][Queen]-1, 0) + max(counts[color][Rook]-2, 0) +
			max(counts[color][Bishop]-2, 0) + max(counts[color][Knight]-2, 0)
		if extra > 8-counts[color][Pawn] {
			return fmt.Errorf("%s has pieces without enough missing pawns for promotion", colorName(color))
		}
	}
	if abs(int(kings[White]%8)-int(kings[Black]%8)) <= 1 && abs(int(kings[White]/8)-int(kings[Black]/8)) <= 1 {
		return fmt.Errorf("kings cannot be adjacent")
	}
	for _, right := range []struct {
		flag       CastlingRights
		king, rook Square
		color      Color
	}{{WhiteKingSide, 4, 7, White}, {WhiteQueenSide, 4, 0, White}, {BlackKingSide, 60, 63, Black}, {BlackQueenSide, 60, 56, Black}} {
		if p.castling&right.flag != 0 && (p.board[right.king] != (Piece{Type: King, Color: right.color}) || p.board[right.rook] != (Piece{Type: Rook, Color: right.color})) {
			return fmt.Errorf("castling right has no king and rook on their starting squares")
		}
	}
	if p.enPassant != NoSquare {
		if err := p.validateEnPassant(); err != nil {
			return err
		}
	}
	if p.inCheck(p.turn.Opponent()) {
		return fmt.Errorf("side not to move is in check")
	}
	return nil
}

func (p Position) validateEnPassant() error {
	wantRank, pawnOffset, sourceOffset := Square(5), -8, 8
	if p.turn == Black {
		wantRank, pawnOffset, sourceOffset = 2, 8, -8
	}
	pawnSquare := Square(int(p.enPassant) + pawnOffset)
	sourceSquare := Square(int(p.enPassant) + sourceOffset)
	if p.enPassant/8 != wantRank || !p.board[p.enPassant].IsEmpty() ||
		p.board[pawnSquare] != (Piece{Type: Pawn, Color: p.turn.Opponent()}) ||
		!p.board[sourceSquare].IsEmpty() || p.halfmoveClock != 0 {
		return fmt.Errorf("en passant target is inconsistent with the previous double pawn move")
	}
	return nil
}

func colorName(color Color) string {
	if color == White {
		return "white"
	}
	return "black"
}

func (p *Position) parseBoard(value string) error {
	ranks := strings.Split(value, "/")
	if len(ranks) != 8 {
		return fmt.Errorf("board must contain 8 ranks")
	}
	for fenRank, rank := range ranks {
		file := 0
		for _, symbol := range rank {
			if symbol >= '1' && symbol <= '8' {
				file += int(symbol - '0')
				continue
			}
			piece, ok := pieceFromFEN(symbol)
			if !ok || file >= 8 {
				return fmt.Errorf("invalid board rank %q", rank)
			}
			p.board[(7-fenRank)*8+file] = piece
			file++
		}
		if file != 8 {
			return fmt.Errorf("invalid board rank %q", rank)
		}
	}
	return nil
}

func (p *Position) parseCastling(value string) error {
	if value == "-" {
		return nil
	}
	for _, symbol := range value {
		var right CastlingRights
		switch symbol {
		case 'K':
			right = WhiteKingSide
		case 'Q':
			right = WhiteQueenSide
		case 'k':
			right = BlackKingSide
		case 'q':
			right = BlackQueenSide
		default:
			return fmt.Errorf("invalid castling rights %q", value)
		}
		if p.castling&right != 0 {
			return fmt.Errorf("duplicate castling right %q", string(symbol))
		}
		p.castling |= right
	}
	return nil
}

func parseUint16(value, name string, allowZero bool) (uint16, error) {
	n, err := strconv.ParseUint(value, 10, 16)
	if err != nil || (!allowZero && n == 0) {
		return 0, fmt.Errorf("invalid %s %q", name, value)
	}
	return uint16(n), nil
}

func pieceFromFEN(symbol rune) (Piece, bool) {
	color := Black
	if symbol >= 'A' && symbol <= 'Z' {
		color, symbol = White, symbol+('a'-'A')
	}
	types := map[rune]PieceType{'p': Pawn, 'n': Knight, 'b': Bishop, 'r': Rook, 'q': Queen, 'k': King}
	pieceType, ok := types[symbol]
	return Piece{Type: pieceType, Color: color}, ok
}

// FEN returns the complete six-field FEN representation of p.
func (p Position) FEN() string {
	var board strings.Builder
	for rank := 7; rank >= 0; rank-- {
		if rank != 7 {
			board.WriteByte('/')
		}
		empty := 0
		for file := 0; file < 8; file++ {
			piece := p.board[rank*8+file]
			if piece.IsEmpty() {
				empty++
				continue
			}
			if empty > 0 {
				board.WriteByte(byte('0' + empty))
				empty = 0
			}
			board.WriteRune(piece.fenSymbol())
		}
		if empty > 0 {
			board.WriteByte(byte('0' + empty))
		}
	}
	turn := "w"
	if p.turn == Black {
		turn = "b"
	}
	castling := ""
	for _, right := range []struct {
		flag   CastlingRights
		symbol byte
	}{{WhiteKingSide, 'K'}, {WhiteQueenSide, 'Q'}, {BlackKingSide, 'k'}, {BlackQueenSide, 'q'}} {
		if p.castling&right.flag != 0 {
			castling += string(right.symbol)
		}
	}
	if castling == "" {
		castling = "-"
	}
	return fmt.Sprintf("%s %s %s %s %d %d", board.String(), turn, castling, p.enPassant, p.halfmoveClock, p.fullmoveNumber)
}

func (p Piece) fenSymbol() rune {
	symbols := [...]rune{0, 'p', 'n', 'b', 'r', 'q', 'k'}
	symbol := symbols[p.Type]
	if p.Color == White {
		symbol -= 'a' - 'A'
	}
	return symbol
}
