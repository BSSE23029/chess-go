package chess

import (
	"fmt"
	"strconv"
	"strings"
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
	return position, nil
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
