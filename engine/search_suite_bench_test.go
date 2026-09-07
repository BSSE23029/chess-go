package engine

import (
	"context"
	"fmt"
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
			var nodes, ttHits, evalHits, pawnHits uint64
			for range b.N {
				_, stats, err := bot.Search(context.Background(), position, SearchLimits{MaxDepth: 3})
				if err != nil {
					b.Fatal(err)
				}
				nodes += stats.Nodes
				ttHits += stats.TTHits
				evalHits += stats.EvalCacheHits
				pawnHits += stats.PawnCacheHits
			}
			b.ReportMetric(float64(nodes)/float64(b.N), "nodes/search")
			b.ReportMetric(float64(ttHits)/float64(b.N), "tt-hits/search")
			b.ReportMetric(float64(evalHits)/float64(b.N), "eval-cache-hits/search")
			b.ReportMetric(float64(pawnHits)/float64(b.N), "pawn-cache-hits/search")
		})
	}
}

// BenchmarkSearchSuiteDepth3TableSizes compares the fixed transposition-table
// capacities used when tuning search memory against node reuse.
func BenchmarkSearchSuiteDepth3TableSizes(b *testing.B) {
	position, err := chess.ParseFEN(chess.InitialFEN)
	if err != nil {
		b.Fatal(err)
	}
	for _, size := range []int{1 << 11, 1 << 12, 1 << 13, 1 << 14} {
		b.Run(formatTableSize(size), func(b *testing.B) {
			bot := New(3)
			bot.TranspositionTableSize = size
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

// BenchmarkStrengthProfiles exercises the real Club, Advanced, and Maximum
// presets without the opening book so profile tuning reflects search work.
func BenchmarkStrengthProfiles(b *testing.B) {
	position, err := chess.ParseFEN(searchBenchmarkPositions[1].fen)
	if err != nil {
		b.Fatal(err)
	}
	for _, profile := range []StrengthProfile{Club, Advanced, Maximum} {
		b.Run(profile.String(), func(b *testing.B) {
			bot := NewProfile(profile)
			bot.Book = nil
			b.ReportAllocs()
			b.ResetTimer()
			var nodes uint64
			for range b.N {
				_, stats, err := bot.Search(context.Background(), position, SearchLimits{MaxDepth: bot.Depth})
				if err != nil {
					b.Fatal(err)
				}
				nodes += stats.Nodes
			}
			b.ReportMetric(float64(nodes)/float64(b.N), "nodes/search")
		})
	}
}

// BenchmarkQuiescenceCaptureScoring keeps SEE's extra cost visible before it
// is considered for every default quiescence capture.
func BenchmarkQuiescenceCaptureScoring(b *testing.B) {
	position, err := chess.ParseFEN("4k3/8/8/8/3q4/8/3R4/4K3 w - - 0 1")
	if err != nil {
		b.Fatal(err)
	}
	var captures []chess.Move
	for _, move := range position.LegalMoves() {
		if move.Flags&chess.Capture != 0 {
			captures = append(captures, move)
		}
	}
	if len(captures) == 0 {
		b.Fatal("benchmark position has no captures")
	}
	b.Run("captured-value", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			_ = captureGain(position, captures[index%len(captures)])
		}
	})
	b.Run("static-exchange", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			_ = staticExchange(position, captures[index%len(captures)])
		}
	})
}

func formatTableSize(size int) string {
	return fmt.Sprintf("tt-%dk", size/1024)
}
