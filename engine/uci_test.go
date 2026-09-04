package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"chess-go"
)

func TestParseBestMove(t *testing.T) {
	if got, err := parseBestMove("info depth 2\nbestmove e2e4 ponder e7e5\n"); err != nil || got != "e2e4" {
		t.Fatalf("parseBestMove() = %q, %v", got, err)
	}
	for _, output := range []string{"", "bestmove (none)"} {
		if _, err := parseBestMove(output); !errors.Is(err, ErrNoBestMove) {
			t.Fatalf("parseBestMove(%q) error = %v", output, err)
		}
	}
}

func TestUCIEngineValidatesBestMove(t *testing.T) {
	t.Setenv("CHESS_GO_UCI_HELPER", "valid")
	adapter, err := NewUCIEngine(os.Args[0], "-test.run=TestUCIHelperProcess")
	if err != nil {
		t.Fatal(err)
	}
	adapter.Depth = 3
	move, err := adapter.ChooseMove(context.Background(), chess.NewPosition())
	if err != nil || move.UCI() != "e2e4" {
		t.Fatalf("UCI move = %s, %v", move.UCI(), err)
	}
	t.Setenv("CHESS_GO_UCI_HELPER", "illegal")
	if _, err := adapter.ChooseMove(context.Background(), chess.NewPosition()); err == nil || !strings.Contains(err.Error(), "illegal move") {
		t.Fatalf("illegal UCI move error = %v", err)
	}
}

func TestUCIEngineConfigurationAndContext(t *testing.T) {
	if _, err := NewUCIEngine(" "); err == nil {
		t.Fatal("blank UCI command accepted")
	}
	adapter, err := NewUCIEngine("missing-engine-for-test")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Depth != 8 {
		t.Fatalf("default UCI depth = %d", adapter.Depth)
	}
	if _, err := adapter.ChooseMove(nil, chess.NewPosition()); err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := adapter.ChooseMove(ctx, chess.NewPosition()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled UCI context error = %v", err)
	}
}

func TestUCIHelperProcess(t *testing.T) {
	mode := os.Getenv("CHESS_GO_UCI_HELPER")
	if mode == "" {
		return
	}
	if mode == "valid" {
		fmt.Fprintln(os.Stdout, "bestmove e2e4")
	} else {
		fmt.Fprintln(os.Stdout, "bestmove e2e5")
	}
	os.Exit(0)
}
