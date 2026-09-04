package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTournamentCLIIncludesConfiguredUCIPlayer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake UCI helper uses a POSIX shell")
	}
	command := filepath.Join(t.TempDir(), "fake-uci")
	script := "#!/bin/sh\n" +
		"turn=w\n" +
		"while IFS= read -r line; do\n" +
		"  case \"$line\" in\n" +
		"    *'position fen '*' b '*) turn=b ;;\n" +
		"  esac\n" +
		"done\n" +
		"if [ \"$turn\" = b ]; then printf 'bestmove e7e5\\n'; else printf 'bestmove e2e4\\n'; fi\n"
	if err := os.WriteFile(command, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	err := run(context.Background(), []string{
		"--profiles", "Learner,Beginner",
		"--uci", command,
		"--uci-name", "FakeUCI",
		"--uci-depth", "1",
		"--games", "1",
		"--plies", "1",
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "FakeUCI:") || !strings.Contains(output.String(), "Learner:") {
		t.Fatalf("UCI tournament output = %s", output.String())
	}
}

func TestTournamentCLIRejectsInvalidUCIConfiguration(t *testing.T) {
	var output strings.Builder
	if err := run(context.Background(), []string{"--uci", "fake", "--uci-depth", "0"}, &output); err == nil || !strings.Contains(err.Error(), "uci-depth") {
		t.Fatalf("invalid UCI depth error = %v", err)
	}
}
