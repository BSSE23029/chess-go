package chess

import (
	"fmt"
	"strconv"
	"strings"
)

type Color uint8

const (
	White Color = iota
	Black
)

func (c Color) Opponent() Color { return c ^ 1 }

type PieceType uint8

const (
	NoPiece PieceType = iota
	Pawn
	Knight
	Bishop
	Rook
	Queen
	King
)

type MoveFlags uint8

const (
	Capture MoveFlags = 1 << iota
	EnPassant
	Castle
	PawnDouble
)

type Move struct {
	From      Square
	To        Square
	Promotion PieceType
	Flags     MoveFlags
}

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

func (m Move) UCI() string {
	value := m.From.String() + m.To.String()
	if m.Promotion != NoPiece {
		value += string(map[PieceType]byte{Queen: 'q', Rook: 'r', Bishop: 'b', Knight: 'n'}[m.Promotion])
	}
	return value
}

type Piece struct {
	Type  PieceType
	Color Color
}

func (p Piece) IsEmpty() bool { return p.Type == NoPiece }

type Square uint8

const NoSquare Square = 64

func ParseSquare(value string) (Square, error) {
	if len(value) != 2 || value[0] < 'a' || value[0] > 'h' || value[1] < '1' || value[1] > '8' {
		return NoSquare, fmt.Errorf("invalid square %q", value)
	}
	return Square((value[1]-'1')*8 + value[0] - 'a'), nil
}

func (s Square) String() string {
	if s > 63 {
		return "-"
	}
	return string([]byte{'a' + byte(s%8), '1' + byte(s/8)})
}

type CastlingRights uint8

const (
	WhiteKingSide CastlingRights = 1 << iota
	WhiteQueenSide
	BlackKingSide
	BlackQueenSide
)

const InitialFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type Position struct {
	board          [64]Piece
	turn           Color
	castling       CastlingRights
	enPassant      Square
	halfmoveClock  uint16
	fullmoveNumber uint16
}

func NewPosition() Position {
	position, err := ParseFEN(InitialFEN)
	if err != nil {
		panic(err)
	}
	return position
}

func (p Position) PieceAt(square Square) Piece {
	if square > 63 {
		return Piece{}
	}
	return p.board[square]
}

func (p Position) Turn() Color              { return p.turn }
func (p Position) Castling() CastlingRights { return p.castling }
func (p Position) EnPassant() Square        { return p.enPassant }
func (p Position) HalfmoveClock() uint16    { return p.halfmoveClock }
func (p Position) FullmoveNumber() uint16   { return p.fullmoveNumber }
func (p Position) InCheck() bool            { return p.inCheck(p.turn) }

// Hash identifies the parts of a position that affect legal play. Move clocks
// are intentionally excluded so equivalent search positions share a key.
func (p Position) Hash() uint64 {
	const (
		offset = uint64(14695981039346656037)
		prime  = uint64(1099511628211)
	)
	hash := offset
	mix := func(value byte) { hash = (hash ^ uint64(value)) * prime }
	for _, piece := range p.board {
		mix(byte(piece.Type) | byte(piece.Color)<<3)
	}
	mix(byte(p.turn))
	mix(byte(p.castling))
	mix(byte(p.enPassant + 1))
	return hash
}

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
