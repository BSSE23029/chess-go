package main

import (
	"bufio"
	"errors"
	"io"
	"os"
)

// launcherSettings exposes runtime values that are intentionally environment-
// backed. The launcher edits the same variables consumed by CLI and network
// boundaries, so a setting selected here has identical behavior when the
// chosen operation starts.
func launcherSettings(reader *bufio.Reader, output io.Writer) error {
	theme, err := launcherPromptChoice(reader, output, "Board theme", firstSet(os.Getenv("CHESS_THEME"), "unicode"), "ascii", "unicode")
	if err != nil {
		return err
	}
	style, err := launcherPromptChoice(reader, output, "Piece style", firstSet(os.Getenv("CHESS_PIECE_STYLE"), "auto"), "auto", "text", "sprite", "emoji")
	if err != nil {
		return err
	}
	random, err := launcherPromptBool(reader, output, "Vary near-best bot moves", envBool("CHESS_BOT_RANDOM", true))
	if err != nil {
		return err
	}
	seed, err := launcherPromptOptional(reader, output, "Bot seed (optional)", os.Getenv("CHESS_BOT_SEED"))
	if err != nil {
		return err
	}
	format, err := launcherPromptChoice(reader, output, "Network wire format", firstSet(os.Getenv("CHESS_NETWORK_FORMAT"), "json"), "json", "protobuf")
	if err != nil {
		return err
	}
	insecure, err := launcherPromptBool(reader, output, "Allow insecure HTTP locally", envBool("CHESS_NETWORK_INSECURE", false))
	if err != nil {
		return err
	}
	ca, err := launcherPromptOptional(reader, output, "TLS CA path (optional)", os.Getenv("CHESS_TLS_CA"))
	if err != nil {
		return err
	}
	clientCertificate, err := launcherPromptOptional(reader, output, "TLS client certificate (optional)", os.Getenv("CHESS_TLS_CLIENT_CERT"))
	if err != nil {
		return err
	}
	clientKey, err := launcherPromptOptional(reader, output, "TLS client private key (optional)", os.Getenv("CHESS_TLS_CLIENT_KEY"))
	if err != nil {
		return err
	}
	if (clientCertificate == "") != (clientKey == "") {
		return errors.New("TLS client certificate and private key must be provided together")
	}
	setLauncherEnv("CHESS_THEME", theme)
	setLauncherEnv("CHESS_PIECE_STYLE", style)
	setLauncherEnv("CHESS_BOT_RANDOM", boolString(random))
	setLauncherEnv("CHESS_BOT_SEED", seed)
	setLauncherEnv("CHESS_NETWORK_FORMAT", format)
	setLauncherEnv("CHESS_NETWORK_INSECURE", boolString(insecure))
	setLauncherEnv("CHESS_TLS_CA", ca)
	setLauncherEnv("CHESS_TLS_CLIENT_CERT", clientCertificate)
	setLauncherEnv("CHESS_TLS_CLIENT_KEY", clientKey)
	return nil
}

func setLauncherEnv(name, value string) {
	if value == "" {
		_ = os.Unsetenv(name)
		return
	}
	_ = os.Setenv(name, value)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
