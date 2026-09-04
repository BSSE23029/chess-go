package chess

import "fmt"

// NullMove passes the turn for search pruning. It is not a legal chess move
// and must never be used to update a Game or a played move history.
func (p Position) NullMove() (Position, error) {
	if p.InCheck() {
		return Position{}, fmt.Errorf("cannot pass while in check")
	}
	p.enPassant = NoSquare
	if p.halfmoveClock < ^uint16(0) {
		p.halfmoveClock++
	}
	if p.turn == Black && p.fullmoveNumber < ^uint16(0) {
		p.fullmoveNumber++
	}
	p.turn = p.turn.Opponent()
	p.hash = p.calculateHash()
	return p, nil
}
