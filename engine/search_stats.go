package engine

func (c *searchControl) updateStats(stats SearchStats) SearchStats {
	stats.Nodes = c.nodes
	stats.ReducedNodes = c.reductions
	stats.NullCutoffs = c.nullCutoffs
	stats.DeltaPrunes = c.deltaPrunes
	stats.TTHits = c.ttHits
	stats.EvalCacheHits = c.evalHits
	stats.PawnCacheHits = c.pawnCache.hits
	return stats
}
