package engine

import "chess-go"

// historyIndex maps a move's origin and destination to a compact, allocation-
// free history bucket. Promotions share the same bucket because the source
// move ordering signal is useful before promotion choice is searched.
func historyIndex(move chess.Move) int {
	return int(move.From)*64 + int(move.To)
}

func (c *searchControl) historyScore(move chess.Move) int {
	if c == nil || move.From >= 64 || move.To >= 64 {
		return 0
	}
	return c.history[historyIndex(move)]
}

func (c *searchControl) recordHistory(move chess.Move, bonus int) {
	if c == nil || move.From >= 64 || move.To >= 64 {
		return
	}
	index := historyIndex(move)
	c.history[index] = minHistoryScore(c.history[index] + bonus)
}

func minHistoryScore(value int) int {
	const maximum = 1 << 20
	if value > maximum {
		return maximum
	}
	return value
}
