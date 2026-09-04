package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCountAndDivide(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"--depth", "2"}, &output); err != nil || output.String() != "400\n" {
		t.Fatalf("count output = %q, %v", output.String(), err)
	}
	output.Reset()
	if err := run(context.Background(), []string{"--depth", "1", "--divide"}, &output); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(output.String(), "\n"); lines != 21 || !strings.Contains(output.String(), "total 20\n") {
		t.Fatalf("divide output has %d lines:\n%s", lines, output.String())
	}
}

func TestHelpListsFlags(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"--help"}, &output); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--fen", "--depth", "--divide"} {
		if !strings.Contains(output.String(), flag[1:]) {
			t.Fatalf("help output missing %q:\n%s", flag, output.String())
		}
	}
}

func TestValidationAndCancellation(t *testing.T) {
	for _, args := range [][]string{{"extra"}, {"--depth", "-1"}, {"--fen", "bad"}, {"--divide", "--depth", "0"}} {
		if err := run(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Errorf("run(%q) succeeded", args)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx, []string{"--depth", "2"}, &bytes.Buffer{}); err == nil {
		t.Fatal("canceled run succeeded")
	}
}
