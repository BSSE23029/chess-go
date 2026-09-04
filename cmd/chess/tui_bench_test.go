package main

import (
	"io"
	"testing"

	"chess-go"
)

func BenchmarkInteractiveRender(b *testing.B) {
	game := chess.NewGame()
	for _, move := range []string{"e2e4", "e7e5", "g1f3", "b8c6"} {
		if err := game.PlayUCI(move); err != nil {
			b.Fatal(err)
		}
	}
	ui := boardUI{cursor: chess.NoSquare, whiteName: "White", blackName: "Black"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ui.invalidate()
		renderInteractive(io.Discard, game, &ui, false, "White 05:00 · Black 05:00 · +00:03", unicodeTheme)
	}
}

func BenchmarkInteractiveClockUpdate(b *testing.B) {
	game := chess.NewGame()
	ui := boardUI{cursor: chess.NoSquare, whiteName: "White", blackName: "Black"}
	renderInteractive(io.Discard, game, &ui, false, "White 05:00 · Black 05:00 · +00:03", unicodeTheme)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		clocks := "White 05:00 · Black 05:00 · +00:03"
		if index%2 == 1 {
			clocks = "White 04:59 · Black 05:00 · +00:03"
		}
		renderInteractive(io.Discard, game, &ui, false, clocks, unicodeTheme)
	}
}
