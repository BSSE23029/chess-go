package engine

import (
	"context"
	"testing"

	"chess-go"
)

func BenchmarkLegalMovesInitial(b *testing.B) {
	position := chess.NewPosition()
	b.ReportAllocs()
	for range b.N {
		_ = position.LegalMoves()
	}
}

func BenchmarkPositionalEvaluation(b *testing.B) {
	position := chess.NewPosition()
	evaluator := PositionalEvaluator{}
	b.ReportAllocs()
	for range b.N {
		_ = evaluator.Evaluate(position)
	}
}

func BenchmarkSearchDepth3(b *testing.B) {
	benchmarkSearch(b, 3)
}

func BenchmarkSearchDepth4(b *testing.B) {
	benchmarkSearch(b, 4)
}

func benchmarkSearch(b *testing.B, depth int) {
	position := chess.NewPosition()
	bot := New(depth)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, _, err := bot.Search(context.Background(), position, SearchLimits{MaxDepth: depth}); err != nil {
			b.Fatal(err)
		}
	}
}
