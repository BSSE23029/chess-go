package engine

import (
	"context"
	"errors"
	"math"
	"slices"
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
	// ReducedNodes is the number of late-move reduced searches attempted.
	ReducedNodes uint64
	// NullCutoffs is the number of branches cut by null-move pruning.
	NullCutoffs uint64
}

type searchControl struct {
	nodes       uint64
	limit       uint64
	table       map[uint64]ttEntry
	killers     [64][2]chess.Move
	history     map[chess.Move]int
	reductions  uint64
	nullCutoffs uint64
	random      uint64
}

type ttBound uint8

const (
	ttExact ttBound = iota
	ttLower
	ttUpper
)

type ttEntry struct {
	depth int
	score Score
	move  chess.Move
	bound ttBound
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
	if move, ok := b.bookMove(position, moves); ok {
		return move, SearchStats{}, nil
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
	control := &searchControl{limit: limits.MaxNodes, table: make(map[uint64]ttEntry), history: make(map[chess.Move]int), random: b.Seed}
	evaluator := b.Evaluator
	if evaluator == nil {
		evaluator = MaterialEvaluator{}
	}
	best := moves[0]
	var stats SearchStats
	for depth := 1; depth <= maxDepth; depth++ {
		alpha, beta := -infinity, infinity
		if stats.Depth > 0 {
			window := Score(50)
			alpha, beta = stats.Score-window, stats.Score+window
		}
		for {
			candidate, score, failLow, failHigh, err := b.iteration(searchCtx, evaluator, &position, moves, depth, alpha, beta, control)
			if err != nil {
				stats.Nodes, stats.ReducedNodes, stats.NullCutoffs = control.nodes, control.reductions, control.nullCutoffs
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return best, stats, err
				}
				if errors.Is(err, ErrSearchLimit) && stats.Depth > 0 {
					return best, stats, nil
				}
				return best, stats, err
			}
			if failLow || failHigh {
				alpha, beta = -infinity, infinity
				continue
			}
			best, stats.Depth, stats.Score = candidate, depth, score
			stats.Nodes, stats.ReducedNodes, stats.NullCutoffs = control.nodes, control.reductions, control.nullCutoffs
			break
		}
	}
	return best, stats, nil
}

func (b *Bot) bookMove(position chess.Position, legal []chess.Move) (chess.Move, bool) {
	if b.Book == nil {
		return chess.Move{}, false
	}
	move, ok := b.Book.Lookup(position)
	if !ok {
		return chess.Move{}, false
	}
	for _, candidate := range legal {
		if candidate.From == move.From && candidate.To == move.To && candidate.Promotion == move.Promotion {
			return candidate, true
		}
	}
	return chess.Move{}, false
}

func (b *Bot) iteration(ctx context.Context, evaluator Evaluator, position *chess.Position, moves []chess.Move, depth int, alpha, beta Score, control *searchControl) (chess.Move, Score, bool, bool, error) {
	moves = orderedSearchMoves(position, 0, control)
	originalAlpha := alpha
	best, bestScore := moves[0], -infinity
	candidates := make([]scoredMove, 0, len(moves))
	for index, move := range moves {
		undo := position.MakeLegalMove(move)
		searchAlpha, searchBeta := -beta, -alpha
		if index > 0 && b.principalVariationSearch() {
			// Most moves are expected to fail below alpha. A null-window probe
			// avoids a full subtree search; only a move that raises alpha is
			// re-searched with the exact window needed for ranking.
			searchAlpha, searchBeta = -alpha-1, -alpha
		}
		score, err := b.search(ctx, evaluator, position, depth-1, 1, searchAlpha, searchBeta, control)
		position.UnmakeMove(undo)
		if err != nil {
			return best, bestScore, false, false, err
		}
		score = -score
		if index > 0 && b.principalVariationSearch() && score > alpha && score < beta {
			undo = position.MakeLegalMove(move)
			score, err = b.search(ctx, evaluator, position, depth-1, 1, -beta, -alpha, control)
			position.UnmakeMove(undo)
			if err != nil {
				return best, bestScore, false, false, err
			}
			score = -score
		}
		candidates = append(candidates, scoredMove{move: move, score: score})
		if score > bestScore {
			best, bestScore = move, score
		}
		if score > alpha {
			alpha = score
		}
		if score >= beta {
			break
		}
	}
	return b.selectCandidate(*position, candidates, bestScore, control), bestScore, bestScore <= originalAlpha, bestScore >= beta, nil
}

func (b *Bot) principalVariationSearch() bool {
	// Imperfect bots need exact root scores to sample a bounded near-best set.
	// Deterministic best-move searches can safely use null-window probes.
	return b.MaxLoss == 0 && b.Temperature <= 0 && b.Seed == 0
}

type scoredMove struct {
	move  chess.Move
	score Score
}

func chooseCandidate(candidates []scoredMove, bestScore Score, temperature float64, control *searchControl) chess.Move {
	if temperature <= 0 || control.random == 0 || len(candidates) == 1 {
		return candidates[0].move
	}
	total := 0.0
	weights := make([]float64, len(candidates))
	for index, candidate := range candidates {
		weight := math.Exp(float64(candidate.score-bestScore) / temperature)
		weights[index], total = weight, total+weight
	}
	target := randomUnit(control) * total
	for index, weight := range weights {
		if target < weight {
			return candidates[index].move
		}
		target -= weight
	}
	return candidates[len(candidates)-1].move
}

func randomUnit(control *searchControl) float64 {
	control.random += 0x9e3779b97f4a7c15
	value := control.random
	value = (value ^ value>>30) * 0xbf58476d1ce4e5b9
	value = (value ^ value>>27) * 0x94d049bb133111eb
	value ^= value >> 31
	return float64(value>>11) / float64(uint64(1)<<53)
}

func (b *Bot) search(ctx context.Context, evaluator Evaluator, position *chess.Position, depth, ply int, alpha, beta Score, control *searchControl) (Score, error) {
	if err := control.visit(ctx); err != nil {
		return 0, err
	}
	if control.history == nil {
		control.history = make(map[chess.Move]int)
	}
	originalAlpha := alpha
	var cached ttEntry
	if depth > 0 && control.table != nil {
		if entry, ok := control.table[position.Hash()]; ok {
			cached = entry
			if entry.depth >= depth {
				switch {
				case entry.bound == ttExact:
					return entry.score, nil
				case entry.bound == ttLower && entry.score >= beta:
					return entry.score, nil
				case entry.bound == ttUpper && entry.score <= alpha:
					return entry.score, nil
				}
			}
		}
	}
	inCheck := position.InCheck()
	if depth >= 3 && !inCheck && canNullMove(position) {
		nullPosition, err := position.NullMove()
		if err != nil {
			return 0, err
		}
		nullDepth := depth - 3
		score, err := b.search(ctx, evaluator, &nullPosition, nullDepth, ply+1, -beta, -beta+1, control)
		if err != nil {
			return 0, err
		}
		if -score >= beta {
			control.nullCutoffs++
			return -score, nil
		}
	}
	moves := orderedSearchMoves(position, ply, control)
	if len(moves) == 0 {
		if position.InCheck() {
			return -MateScore + Score(ply), nil
		}
		return 0, nil
	}
	if depth == 0 {
		return b.quiescence(ctx, evaluator, position, ply, alpha, beta, control)
	}
	if cached.move != (chess.Move{}) {
		moves = prioritizeMove(moves, cached.move)
	}
	best := -infinity
	bestMove := moves[0]
	for index, move := range moves {
		undo := position.MakeLegalMove(move)
		reduced := depth >= 3 && index >= 3 && !inCheck && move.Flags&chess.Capture == 0 && move.Promotion == chess.NoPiece
		if reduced {
			control.reductions++
		}
		searchDepth := depth - 1
		searchAlpha, searchBeta := -beta, -alpha
		if reduced {
			searchDepth = depth - 2
			searchAlpha, searchBeta = -alpha-1, -alpha
		}
		score, err := b.search(ctx, evaluator, position, searchDepth, ply+1, searchAlpha, searchBeta, control)
		if err == nil && reduced && -score > alpha {
			score, err = b.search(ctx, evaluator, position, depth-1, ply+1, -beta, -alpha, control)
		}
		position.UnmakeMove(undo)
		if err != nil {
			return 0, err
		}
		score = -score
		if score > best {
			best = score
			bestMove = move
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			if move.Flags&chess.Capture == 0 {
				control.history[move] += depth * depth
				if ply < len(control.killers) {
					if control.killers[ply][0] != move {
						control.killers[ply][1] = control.killers[ply][0]
						control.killers[ply][0] = move
					}
				}
			}
			break
		}
	}
	bound := ttExact
	if best <= originalAlpha {
		bound = ttUpper
	} else if best >= beta {
		bound = ttLower
	}
	return b.store(position, depth, best, bestMove, bound, control), nil
}

func canNullMove(position *chess.Position) bool {
	for square := chess.Square(0); square < 64; square++ {
		piece := position.PieceAt(square)
		if piece.Color == position.Turn() && piece.Type != chess.Pawn && piece.Type != chess.King && !piece.IsEmpty() {
			return true
		}
	}
	return false
}

func (b *Bot) store(position *chess.Position, depth int, score Score, move chess.Move, bound ttBound, control *searchControl) Score {
	if control.table != nil {
		control.table[position.Hash()] = ttEntry{depth: depth, score: score, move: move, bound: bound}
	}
	return score
}

func prioritizeMove(moves []chess.Move, preferred chess.Move) []chess.Move {
	for index, move := range moves {
		if move == preferred {
			moves[0], moves[index] = moves[index], moves[0]
			break
		}
	}
	return moves
}

func orderedSearchMoves(position *chess.Position, ply int, control *searchControl) []chess.Move {
	moves := orderedMoves(position)
	if control == nil {
		return moves
	}
	slices.SortStableFunc(moves, func(a, b chess.Move) int {
		return searchMovePriority(position, a, ply, control) - searchMovePriority(position, b, ply, control)
	})
	return moves
}

func searchMovePriority(position *chess.Position, move chess.Move, ply int, control *searchControl) int {
	priority := movePriority(position, move)
	if ply < len(control.killers) {
		if control.killers[ply][0] == move {
			priority += 10000
		} else if control.killers[ply][1] == move {
			priority += 5000
		}
	}
	priority += control.history[move] / 100
	return priority
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
