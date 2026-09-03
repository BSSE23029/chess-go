package chess

import "fmt"

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
