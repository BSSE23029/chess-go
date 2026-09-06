package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"chess-go/engine"
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
	playerName, err := launcherPromptOptional(reader, output, "Player name (optional)", os.Getenv("CHESS_PLAYER_NAME"))
	if err != nil {
		return err
	}
	playerColor, err := launcherPromptChoice(reader, output, "Default player color", firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white"), "white", "black")
	if err != nil {
		return err
	}
	botName, err := launcherPromptOptional(reader, output, "Bot name (optional)", os.Getenv("CHESS_BOT_NAME"))
	if err != nil {
		return err
	}
	depthDefault := firstSet(os.Getenv("CHESS_BOT_DEPTH"), "3")
	depth, err := launcherPromptInt(reader, output, "Bot search depth", depthDefault, 1)
	if err != nil {
		return err
	}
	level, err := launcherPromptOptional(reader, output, "Bot strength profile (optional)", os.Getenv("CHESS_BOT_LEVEL"))
	if err != nil {
		return err
	}
	if level != "" {
		profile, profileErr := engine.ParseStrengthProfile(level)
		if profileErr != nil {
			return fmt.Errorf("bot strength profile: %w", profileErr)
		}
		level = profile.String()
	}
	personality, err := launcherPromptOptional(reader, output, "Bot personality (optional)", os.Getenv("CHESS_BOT_PERSONALITY"))
	if err != nil {
		return err
	}
	if personality != "" {
		style, styleErr := engine.ParsePersonality(personality)
		if styleErr != nil {
			return fmt.Errorf("bot personality: %w", styleErr)
		}
		personality = style.String()
	}
	seed, err := launcherPromptOptional(reader, output, "Bot seed (optional)", os.Getenv("CHESS_BOT_SEED"))
	if err != nil {
		return err
	}
	if seed != "" {
		if _, seedErr := strconv.ParseUint(seed, 0, 64); seedErr != nil {
			return errors.New("bot seed must be an unsigned integer")
		}
	}
	random, err := launcherPromptBool(reader, output, "Vary near-best bot moves", envBool("CHESS_BOT_RANDOM", true))
	if err != nil {
		return err
	}
	clock, err := launcherPromptOptional(reader, output, "Local clock (optional)", os.Getenv("CHESS_CLOCK"))
	if err != nil {
		return err
	}
	increment, err := launcherPromptOptional(reader, output, "Local increment (optional)", os.Getenv("CHESS_INCREMENT"))
	if err != nil {
		return err
	}
	if _, clockErr := parseClock(clock, firstSet(increment, "0s")); clockErr != nil {
		return fmt.Errorf("local clock settings: %w", clockErr)
	}
	networkURL, err := launcherPrompt(reader, output, "Default server URL", firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"))
	if err != nil {
		return err
	}
	networkAddress, err := launcherPrompt(reader, output, "Host listen address", firstSet(os.Getenv("CHESS_NETWORK_ADDR"), ":8080"))
	if err != nil {
		return err
	}
	token, err := launcherPromptSecret(reader, output, "Network bearer token", os.Getenv("CHESS_NETWORK_TOKEN"))
	if err != nil {
		return err
	}
	format, err := launcherPromptChoice(reader, output, "Network wire format", firstSet(os.Getenv("CHESS_NETWORK_FORMAT"), "json"), "json", "protobuf")
	if err != nil {
		return err
	}
	matchID, err := launcherPromptOptional(reader, output, "Default match ID (optional)", os.Getenv("CHESS_MATCH_ID"))
	if err != nil {
		return err
	}
	playerID, err := launcherPromptOptional(reader, output, "Default player ID (optional)", os.Getenv("CHESS_PLAYER_ID"))
	if err != nil {
		return err
	}
	certificate, err := launcherPromptOptional(reader, output, "Server TLS certificate (optional)", os.Getenv("CHESS_TLS_CERT"))
	if err != nil {
		return err
	}
	key, err := launcherPromptOptional(reader, output, "Server TLS private key (optional)", os.Getenv("CHESS_TLS_KEY"))
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
	store, err := launcherPromptOptional(reader, output, "Match store path (optional)", os.Getenv("CHESS_MATCH_STORE"))
	if err != nil {
		return err
	}
	lan, err := launcherPromptBool(reader, output, "Advertise on LAN", envBool("CHESS_LAN_DISCOVERY", false))
	if err != nil {
		return err
	}
	lanInstance, err := launcherPrompt(reader, output, "LAN instance", firstSet(os.Getenv("CHESS_LAN_INSTANCE"), "chess-go"))
	if err != nil {
		return err
	}
	lanHost, err := launcherPromptOptional(reader, output, "LAN advertised host (optional)", os.Getenv("CHESS_LAN_HOST"))
	if err != nil {
		return err
	}
	_, noColorSet := os.LookupEnv("NO_COLOR")
	noColor, err := launcherPromptBool(reader, output, "Disable ANSI color", noColorSet)
	if err != nil {
		return err
	}
	setLauncherEnv("CHESS_THEME", theme)
	setLauncherEnv("CHESS_PIECE_STYLE", style)
	setLauncherEnv("CHESS_PLAYER_NAME", playerName)
	setLauncherEnv("CHESS_PLAYER_COLOR", playerColor)
	setLauncherEnv("CHESS_BOT_NAME", botName)
	setLauncherEnv("CHESS_BOT_DEPTH", depth)
	setLauncherEnv("CHESS_BOT_LEVEL", level)
	setLauncherEnv("CHESS_BOT_PERSONALITY", personality)
	setLauncherEnv("CHESS_BOT_RANDOM", boolString(random))
	setLauncherEnv("CHESS_BOT_SEED", seed)
	setLauncherEnv("CHESS_CLOCK", clock)
	setLauncherEnv("CHESS_INCREMENT", increment)
	setLauncherEnv("CHESS_NETWORK_URL", networkURL)
	setLauncherEnv("CHESS_NETWORK_ADDR", networkAddress)
	setLauncherEnv("CHESS_NETWORK_TOKEN", token)
	setLauncherEnv("CHESS_NETWORK_FORMAT", format)
	setLauncherEnv("CHESS_NETWORK_INSECURE", boolString(insecure))
	setLauncherEnv("CHESS_MATCH_ID", matchID)
	setLauncherEnv("CHESS_PLAYER_ID", playerID)
	setLauncherEnv("CHESS_TLS_CERT", certificate)
	setLauncherEnv("CHESS_TLS_KEY", key)
	setLauncherEnv("CHESS_TLS_CA", ca)
	setLauncherEnv("CHESS_TLS_CLIENT_CERT", clientCertificate)
	setLauncherEnv("CHESS_TLS_CLIENT_KEY", clientKey)
	setLauncherEnv("CHESS_MATCH_STORE", store)
	setLauncherEnv("CHESS_LAN_DISCOVERY", boolString(lan))
	setLauncherEnv("CHESS_LAN_INSTANCE", lanInstance)
	setLauncherEnv("CHESS_LAN_HOST", lanHost)
	if noColor {
		setLauncherEnv("NO_COLOR", "1")
	} else {
		setLauncherEnv("NO_COLOR", "")
	}
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
