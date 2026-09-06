package engine

import "chess-go"

type evaluationEntry struct {
	key   uint64
	score Score
	valid bool
}

type ttEntry struct {
	key   uint64
	depth int
	score Score
	move  chess.Move
	bound ttBound
	valid bool
}

// transpositionTable is a fixed-size, power-of-two table used for one search.
// A bounded table avoids map growth and hashing allocations in the hot path;
// deeper entries are preferred when two positions map to the same slot.
type transpositionTable struct {
	entries []ttEntry
	mask    uint64
}

func newTranspositionTable(size int) *transpositionTable {
	if size < 1 {
		size = 1
	}
	capacity := 1
	for capacity < size {
		capacity <<= 1
	}
	return &transpositionTable{entries: make([]ttEntry, capacity), mask: uint64(capacity - 1)}
}

func (t *transpositionTable) lookup(key uint64) (ttEntry, bool) {
	if t == nil || len(t.entries) == 0 {
		return ttEntry{}, false
	}
	entry := t.entries[key&t.mask]
	return entry, entry.valid && entry.key == key
}

func (t *transpositionTable) store(key uint64, entry ttEntry) {
	if t == nil || len(t.entries) == 0 {
		return
	}
	index := key & t.mask
	current := t.entries[index]
	if current.valid && current.key != key && current.depth > entry.depth {
		return
	}
	entry.key, entry.valid = key, true
	t.entries[index] = entry
}

func (c *searchControl) lookup(key uint64) (ttEntry, bool) {
	if c == nil {
		return ttEntry{}, false
	}
	if c.tt != nil {
		entry, ok := c.tt.lookup(key)
		if ok {
			c.ttHits++
		}
		return entry, ok
	}
	if c.table == nil {
		return ttEntry{}, false
	}
	entry, ok := c.table[key]
	if ok {
		c.ttHits++
	}
	return entry, ok
}

func (c *searchControl) evaluate(evaluator Evaluator, position chess.Position) Score {
	if !cacheableEvaluator(evaluator) {
		return evaluator.Evaluate(position)
	}
	key := position.Hash()
	entry := &c.evalCache[key&(uint64(len(c.evalCache))-1)]
	if entry.valid && entry.key == key {
		return entry.score
	}
	score := evaluator.Evaluate(position)
	*entry = evaluationEntry{key: key, score: score, valid: true}
	return score
}

func cacheableEvaluator(evaluator Evaluator) bool {
	switch evaluator.(type) {
	case MaterialEvaluator, *MaterialEvaluator, PositionalEvaluator, *PositionalEvaluator, EndgameEvaluator, *EndgameEvaluator:
		return true
	default:
		return false
	}
}
