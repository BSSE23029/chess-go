package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"

	"chess-go"
	"chess-go/perft"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "perft:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	options := flag.NewFlagSet("perft", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	fen := options.String("fen", chess.InitialFEN, "position in FEN")
	depth := options.Int("depth", 4, "search depth")
	divide := options.Bool("divide", false, "show each root move")
	if err := options.Parse(args); err != nil || options.NArg() != 0 {
		return errors.New("usage: perft [--fen FEN] [--depth N] [--divide]")
	}
	position, err := chess.ParseFEN(*fen)
	if err != nil {
		return fmt.Errorf("FEN: %w", err)
	}
	if *divide {
		results, err := perft.Divide(ctx, position, *depth)
		if err != nil {
			return err
		}
		var total uint64
		for _, result := range results {
			fmt.Fprintf(output, "%s %d\n", result.Move.UCI(), result.Nodes)
			if math.MaxUint64-total < result.Nodes {
				return perft.ErrOverflow
			}
			total += result.Nodes
		}
		fmt.Fprintln(output, "total", total)
		return nil
	}
	nodes, err := perft.Count(ctx, position, *depth)
	if err != nil {
		return err
	}
	fmt.Fprintln(output, nodes)
	return nil
}
