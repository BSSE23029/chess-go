// Package perft counts legal move-tree nodes for chess-rule verification.
package perft

import (
	"context"
	"errors"
	"math"

	"chess-go"
)

var ErrOverflow = errors.New("perft node count overflowed uint64")

// Result reports the node count beneath one legal root move.
type Result struct {
	Move  chess.Move
	Nodes uint64
}

// Count returns the number of leaf nodes at depth without mutating position.
func Count(ctx context.Context, position chess.Position, depth int) (uint64, error) {
	if depth < 0 {
		return 0, errors.New("perft depth cannot be negative")
	}
	return count(ctx, &position, depth)
}

// Divide returns a separate node count for every legal root move.
func Divide(ctx context.Context, position chess.Position, depth int) ([]Result, error) {
	if depth < 1 {
		return nil, errors.New("perft divide depth must be at least one")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	moves := position.LegalMoves()
	results := make([]Result, 0, len(moves))
	for _, move := range moves {
		undo := position.MakeLegalMove(move)
		nodes, err := count(ctx, &position, depth-1)
		position.UnmakeMove(undo)
		if err != nil {
			return nil, err
		}
		results = append(results, Result{Move: move, Nodes: nodes})
	}
	return results, nil
}

func count(ctx context.Context, position *chess.Position, depth int) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if depth == 0 {
		return 1, nil
	}
	var nodes uint64
	for _, move := range position.LegalMoves() {
		undo := position.MakeLegalMove(move)
		child, err := count(ctx, position, depth-1)
		position.UnmakeMove(undo)
		if err != nil {
			return 0, err
		}
		if math.MaxUint64-nodes < child {
			return 0, ErrOverflow
		}
		nodes += child
	}
	return nodes, nil
}
