package engine

import (
	"context"
	"errors"
	"time"

	"chess-go"
)

// ErrSearchLimit indicates that a search could not complete its first
// iteration within the requested node budget.
var ErrSearchLimit = errors.New("search limit reached")

// SearchLimits bounds an iterative search. A zero bound means unlimited.
type SearchLimits struct {
	// MaxDepth is the maximum depth in plies; zero uses the bot's Depth.
	MaxDepth int
	// MaxNodes is the maximum number of visited search nodes.
	MaxNodes uint64
	// Time is the maximum wall-clock duration; zero means unlimited.
	Time time.Duration
}

// SearchStats reports the completed depth, score, and visited-node count.
type SearchStats struct {
	// Depth is the deepest fully completed iteration.
	Depth int
	// Nodes is the number of visited search nodes.
	Nodes uint64
	// Score is the score of the returned move at Depth.
	Score Score
}

type searchControl struct {
	nodes uint64
	limit uint64
}

func (c *searchControl) visit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.limit != 0 && c.nodes >= c.limit {
		return ErrSearchLimit
	}
	c.nodes++
	return nil
}

// ChooseMove searches position using iterative deepening up to the bot's
// configured depth. Use Search when explicit limits or statistics are needed.
func (b *Bot) ChooseMove(ctx context.Context, position chess.Position) (chess.Move, error) {
	move, _, err := b.Search(ctx, position, SearchLimits{})
	return move, err
}

// Search returns the best move found under limits and its statistics. If a
// limit interrupts an iteration after at least one complete depth, the last
// complete result is returned with a nil error. Context cancellation is always
// returned to the caller.
func (b *Bot) Search(ctx context.Context, position chess.Position, limits SearchLimits) (chess.Move, SearchStats, error) {
	if err := ctx.Err(); err != nil {
		return chess.Move{}, SearchStats{}, err
	}
	moves := orderedMoves(&position)
	if len(moves) == 0 {
		return chess.Move{}, SearchStats{}, ErrNoLegalMoves
	}
	maxDepth := limits.MaxDepth
	if maxDepth < 1 {
		maxDepth = b.Depth
	}
	if maxDepth < 1 {
		maxDepth = 1
	}
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if limits.Time > 0 {
		searchCtx, cancel = context.WithTimeout(ctx, limits.Time)
		defer cancel()
	}
	control := &searchControl{limit: limits.MaxNodes}
	evaluator := b.Evaluator
	if evaluator == nil {
		evaluator = MaterialEvaluator{}
	}
	best := moves[0]
	var stats SearchStats
	for depth := 1; depth <= maxDepth; depth++ {
		candidate, score, err := b.iteration(searchCtx, evaluator, &position, moves, depth, control)
		if err != nil {
			stats.Nodes = control.nodes
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return best, stats, err
			}
			if errors.Is(err, ErrSearchLimit) && stats.Depth > 0 {
				return best, stats, nil
			}
			return best, stats, err
		}
		best, stats.Depth, stats.Score = candidate, depth, score
		stats.Nodes = control.nodes
	}
	return best, stats, nil
}

func (b *Bot) iteration(ctx context.Context, evaluator Evaluator, position *chess.Position, moves []chess.Move, depth int, control *searchControl) (chess.Move, Score, error) {
	best, bestScore := moves[0], -infinity
	type candidate struct {
		move  chess.Move
		score Score
	}
	candidates := make([]candidate, 0, len(moves))
	for _, move := range moves {
		undo := position.MakeLegalMove(move)
		score, err := b.search(ctx, evaluator, position, depth-1, 1, -infinity, infinity, control)
		position.UnmakeMove(undo)
		if err != nil {
			return best, bestScore, err
		}
		score = -score
		candidates = append(candidates, candidate{move: move, score: score})
		if score > bestScore {
			best, bestScore = move, score
		}
	}
	threshold := bestScore - b.MaxLoss
	for _, candidate := range candidates {
		if candidate.score >= threshold {
			return candidate.move, bestScore, nil
		}
	}
	return best, bestScore, nil
}

func (b *Bot) search(ctx context.Context, evaluator Evaluator, position *chess.Position, depth, ply int, alpha, beta Score, control *searchControl) (Score, error) {
	if err := control.visit(ctx); err != nil {
		return 0, err
	}
	moves := orderedMoves(position)
	if len(moves) == 0 {
		if position.InCheck() {
			return -MateScore + Score(ply), nil
		}
		return 0, nil
	}
	if depth == 0 {
		return b.quiescence(ctx, evaluator, position, ply, alpha, beta, control)
	}
	best := -infinity
	for _, move := range moves {
		undo := position.MakeLegalMove(move)
		score, err := b.search(ctx, evaluator, position, depth-1, ply+1, -beta, -alpha, control)
		position.UnmakeMove(undo)
		if err != nil {
			return 0, err
		}
		score = -score
		if score > best {
			best = score
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			break
		}
	}
	return best, nil
}

func (b *Bot) quiescence(ctx context.Context, evaluator Evaluator, position *chess.Position, ply int, alpha, beta Score, control *searchControl) (Score, error) {
	if err := control.visit(ctx); err != nil {
		return 0, err
	}
	moves := orderedMoves(position)
	if len(moves) == 0 {
		if position.InCheck() {
			return -MateScore + Score(ply), nil
		}
		return 0, nil
	}
	inCheck := position.InCheck()
	if !inCheck {
		standPat := evaluator.Evaluate(*position)
		if position.Turn() == chess.Black {
			standPat = -standPat
		}
		if standPat >= beta {
			return standPat, nil
		}
		if standPat > alpha {
			alpha = standPat
		}
	}
	for _, move := range moves {
		if !inCheck && move.Flags&chess.Capture == 0 && move.Promotion == chess.NoPiece {
			continue
		}
		undo := position.MakeLegalMove(move)
		score, err := b.quiescence(ctx, evaluator, position, ply+1, -beta, -alpha, control)
		position.UnmakeMove(undo)
		if err != nil {
			return 0, err
		}
		score = -score
		if score >= beta {
			return score, nil
		}
		if score > alpha {
			alpha = score
		}
	}
	return alpha, nil
}
