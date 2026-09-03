package chess

import "fmt"

type Color uint8

const (
	White Color = iota
	Black
)

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
