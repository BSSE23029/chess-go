package engine

import (
	"context"
	"testing"

	"chess-go"
)

var searchBenchmarkPositions = []struct {
	name string
	fen  string
}{
	{name: "opening", fen: chess.InitialFEN},
	{name: "tactical", fen: "r1bqk2r/pppp1ppp/2n2n2/4p3/3PP3/2B2N2/PPP2PPP/RNBQ1RK1 w kq - 4 6"},
	{name: "quiet", fen: "4k3/8/8/3pp3/3PP3/8/8/4K3 w - - 0 1"},
	{name: "endgame", fen: "8/5pk1/6p1/3P4/3K4/8/8/8 w - - 0 1"},
}

// BenchmarkSearchSuiteDepth3 keeps the optimization baseline representative:
// a single opening position can hide tactical and endgame allocation costs.
func BenchmarkSearchSuiteDepth3(b *testing.B) {
	for _, test := range searchBenchmarkPositions {
		b.Run(test.name, func(b *testing.B) {
			position, err := chess.ParseFEN(test.fen)
			if err != nil {
				b.Fatal(err)
			}
			bot := New(3)
			b.ReportAllocs()
			b.ResetTimer()
			var nodes, ttHits uint64
			for range b.N {
				_, stats, err := bot.Search(context.Background(), position, SearchLimits{MaxDepth: 3})
				if err != nil {
					b.Fatal(err)
				}
				nodes += stats.Nodes
				ttHits += stats.TTHits
			}
			b.ReportMetric(float64(nodes)/float64(b.N), "nodes/search")
			b.ReportMetric(float64(ttHits)/float64(b.N), "tt-hits/search")
		})
	}
}
