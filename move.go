package chess

import "fmt"

var (
	knightSteps = [][2]int{{1, 2}, {2, 1}, {2, -1}, {1, -2}, {-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}}
	kingSteps   = [][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}
	bishopSteps = [][2]int{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}}
	rookSteps   = [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
)

func (p Position) LegalMoves() []Move {
	moves := p.pseudoMoves()
	legal := moves[:0]
	for _, move := range moves {
		next := p.applyUnchecked(move)
		if !next.inCheck(p.turn) {
			legal = append(legal, move)
		}
	}
	return legal
}

func (p Position) Apply(move Move) (Position, error) {
	for _, legal := range p.LegalMoves() {
		if legal.From == move.From && legal.To == move.To && legal.Promotion == move.Promotion {
			return p.applyUnchecked(legal), nil
		}
	}
	return p, fmt.Errorf("illegal move %s", move.UCI())
}

func (p Position) ApplyUCI(value string) (Position, error) {
	move, err := ParseUCI(value)
	if err != nil {
		return p, err
	}
	return p.Apply(move)
}

func (p Position) pseudoMoves() []Move {
	moves := make([]Move, 0, 40)
	for from, piece := range p.board {
		if piece.IsEmpty() || piece.Color != p.turn {
			continue
		}
		square := Square(from)
		switch piece.Type {
		case Pawn:
			moves = p.pawnMoves(moves, square, piece.Color)
		case Knight:
			moves = p.stepMoves(moves, square, knightSteps)
		case Bishop:
			moves = p.slideMoves(moves, square, bishopSteps)
		case Rook:
			moves = p.slideMoves(moves, square, rookSteps)
		case Queen:
			moves = p.slideMoves(moves, square, append(bishopSteps, rookSteps...))
		case King:
			moves = p.stepMoves(moves, square, kingSteps)
			moves = p.castleMoves(moves, square, piece.Color)
		}
	}
	return moves
}

func (p Position) pawnMoves(moves []Move, from Square, color Color) []Move {
	direction, startRank, promotionRank := 1, 1, 7
	if color == Black {
		direction, startRank, promotionRank = -1, 6, 0
	}
	file, rank := int(from%8), int(from/8)
	if to, ok := squareAt(file, rank+direction); ok && p.board[to].IsEmpty() {
		moves = addPawnMove(moves, from, to, promotionRank, 0)
		if rank == startRank {
			double, _ := squareAt(file, rank+2*direction)
			if p.board[double].IsEmpty() {
				moves = append(moves, Move{From: from, To: double, Flags: PawnDouble})
			}
		}
	}
	for _, delta := range []int{-1, 1} {
		to, ok := squareAt(file+delta, rank+direction)
		if !ok {
			continue
		}
		if target := p.board[to]; !target.IsEmpty() && target.Color != color {
			moves = addPawnMove(moves, from, to, promotionRank, Capture)
		} else if to == p.enPassant {
			moves = append(moves, Move{From: from, To: to, Flags: Capture | EnPassant})
		}
	}
	return moves
}

func addPawnMove(moves []Move, from, to Square, promotionRank int, flags MoveFlags) []Move {
	if int(to/8) != promotionRank {
		return append(moves, Move{From: from, To: to, Flags: flags})
	}
	for _, promotion := range []PieceType{Queen, Rook, Bishop, Knight} {
		moves = append(moves, Move{From: from, To: to, Promotion: promotion, Flags: flags})
	}
	return moves
}

func (p Position) stepMoves(moves []Move, from Square, steps [][2]int) []Move {
	file, rank, piece := int(from%8), int(from/8), p.board[from]
	for _, step := range steps {
		to, ok := squareAt(file+step[0], rank+step[1])
		if !ok || (!p.board[to].IsEmpty() && p.board[to].Color == piece.Color) {
			continue
		}
		flags := MoveFlags(0)
		if !p.board[to].IsEmpty() {
			flags = Capture
		}
		moves = append(moves, Move{From: from, To: to, Flags: flags})
	}
	return moves
}

func (p Position) slideMoves(moves []Move, from Square, directions [][2]int) []Move {
	file, rank, piece := int(from%8), int(from/8), p.board[from]
	for _, direction := range directions {
		for distance := 1; ; distance++ {
			to, ok := squareAt(file+direction[0]*distance, rank+direction[1]*distance)
			if !ok {
				break
			}
			target := p.board[to]
			if !target.IsEmpty() && target.Color == piece.Color {
				break
			}
			flags := MoveFlags(0)
			if !target.IsEmpty() {
				flags = Capture
			}
			moves = append(moves, Move{From: from, To: to, Flags: flags})
			if !target.IsEmpty() {
				break
			}
		}
	}
	return moves
}

func (p Position) castleMoves(moves []Move, from Square, color Color) []Move {
	kingStart, rank, kingSide, queenSide := Square(4), 0, WhiteKingSide, WhiteQueenSide
	if color == Black {
		kingStart, rank, kingSide, queenSide = 60, 7, BlackKingSide, BlackQueenSide
	}
	if from != kingStart || p.inCheck(color) {
		return moves
	}
	for _, option := range []struct {
		right CastlingRights
		rook  int
		empty []int
		path  []int
		to    int
	}{{kingSide, 7, []int{5, 6}, []int{5, 6}, 6}, {queenSide, 0, []int{1, 2, 3}, []int{3, 2}, 2}} {
		if p.castling&option.right == 0 || p.board[rank*8+option.rook] != (Piece{Type: Rook, Color: color}) {
			continue
		}
		clear := true
		for _, file := range option.empty {
			clear = clear && p.board[rank*8+file].IsEmpty()
		}
		for _, file := range option.path {
			clear = clear && !p.attacked(Square(rank*8+file), color.Opponent())
		}
		if clear {
			moves = append(moves, Move{From: from, To: Square(rank*8 + option.to), Flags: Castle})
		}
	}
	return moves
}

func (p Position) inCheck(color Color) bool {
	for square, piece := range p.board {
		if piece == (Piece{Type: King, Color: color}) {
			return p.attacked(Square(square), color.Opponent())
		}
	}
	return true
}

func (p Position) attacked(target Square, by Color) bool {
	for from, piece := range p.board {
		if piece.IsEmpty() || piece.Color != by {
			continue
		}
		ff, fr, tf, tr := from%8, from/8, int(target%8), int(target/8)
		df, dr := tf-ff, tr-fr
		switch piece.Type {
		case Pawn:
			direction := 1
			if by == Black {
				direction = -1
			}
			if dr == direction && (df == 1 || df == -1) {
				return true
			}
		case Knight:
			if (abs(df) == 1 && abs(dr) == 2) || (abs(df) == 2 && abs(dr) == 1) {
				return true
			}
		case King:
			if abs(df) <= 1 && abs(dr) <= 1 {
				return true
			}
		case Bishop:
			if abs(df) == abs(dr) && p.clearLine(from, int(target)) {
				return true
			}
		case Rook:
			if (df == 0 || dr == 0) && p.clearLine(from, int(target)) {
				return true
			}
		case Queen:
			if (df == 0 || dr == 0 || abs(df) == abs(dr)) && p.clearLine(from, int(target)) {
				return true
			}
		}
	}
	return false
}

func (p Position) clearLine(from, to int) bool {
	df, dr := sign(to%8-from%8), sign(to/8-from/8)
	for file, rank := from%8+df, from/8+dr; file != to%8 || rank != to/8; file, rank = file+df, rank+dr {
		if !p.board[rank*8+file].IsEmpty() {
			return false
		}
	}
	return true
}

func (p Position) applyUnchecked(move Move) Position {
	next, piece, captured := p, p.board[move.From], p.board[move.To]
	next.board[move.From] = Piece{}
	next.board[move.To] = piece
	if move.Flags&EnPassant != 0 {
		capture := int(move.To) - 8
		if piece.Color == Black {
			capture = int(move.To) + 8
		}
		next.board[capture], captured = Piece{}, Piece{Type: Pawn, Color: piece.Color.Opponent()}
	}
	if move.Promotion != NoPiece {
		next.board[move.To].Type = move.Promotion
	}
	if move.Flags&Castle != 0 {
		if move.To%8 == 6 {
			next.board[move.To-1], next.board[move.To+1] = next.board[move.To+1], Piece{}
		} else {
			next.board[move.To+1], next.board[move.To-2] = next.board[move.To-2], Piece{}
		}
	}
	next.updateCastling(move, piece, captured)
	next.enPassant = NoSquare
	if move.Flags&PawnDouble != 0 {
		next.enPassant = Square((int(move.From) + int(move.To)) / 2)
	}
	next.halfmoveClock++
	if piece.Type == Pawn || !captured.IsEmpty() {
		next.halfmoveClock = 0
	}
	if piece.Color == Black {
		next.fullmoveNumber++
	}
	next.turn = p.turn.Opponent()
	return next
}

func (p *Position) updateCastling(move Move, piece, captured Piece) {
	if piece.Type == King {
		if piece.Color == White {
			p.castling &^= WhiteKingSide | WhiteQueenSide
		} else {
			p.castling &^= BlackKingSide | BlackQueenSide
		}
	}
	for square, right := range map[Square]CastlingRights{0: WhiteQueenSide, 7: WhiteKingSide, 56: BlackQueenSide, 63: BlackKingSide} {
		if move.From == square || (move.To == square && captured.Type == Rook) {
			p.castling &^= right
		}
	}
}

func squareAt(file, rank int) (Square, bool) {
	if file < 0 || file > 7 || rank < 0 || rank > 7 {
		return NoSquare, false
	}
	return Square(rank*8 + file), true
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func sign(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}
