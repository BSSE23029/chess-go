package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func launcherLocal(reader *bufio.Reader, output io.Writer) ([]string, error) {
	clock, err := launcherPrompt(reader, output, "Clock (blank disables)", os.Getenv("CHESS_CLOCK"))
	if err != nil {
		return nil, err
	}
	increment, err := launcherPrompt(reader, output, "Increment", os.Getenv("CHESS_INCREMENT"))
	if err != nil {
		return nil, err
	}
	theme, err := launcherPromptChoice(reader, output, "Theme", firstSet(os.Getenv("CHESS_THEME"), "unicode"), "ascii", "unicode")
	if err != nil {
		return nil, err
	}
	args := []string{"play", "local"}
	args = launcherAppend(args, "--clock", clock)
	args = launcherAppend(args, "--increment", increment)
	args = launcherAppend(args, "--theme", theme)
	return args, nil
}

func launcherBot(reader *bufio.Reader, output io.Writer) ([]string, error) {
	level, err := launcherPromptOptional(reader, output, "Strength level (or - for depth)", os.Getenv("CHESS_BOT_LEVEL"))
	if err != nil {
		return nil, err
	}
	depth, err := launcherPromptInt(reader, output, "Search depth", firstSet(os.Getenv("CHESS_BOT_DEPTH"), "3"), 1)
	if err != nil {
		return nil, err
	}
	color, err := launcherPromptChoice(reader, output, "Human color", firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white"), "white", "black")
	if err != nil {
		return nil, err
	}
	personality, err := launcherPromptOptional(reader, output, "Personality (optional)", os.Getenv("CHESS_BOT_PERSONALITY"))
	if err != nil {
		return nil, err
	}
	seed, err := launcherPromptOptional(reader, output, "Seed (optional)", os.Getenv("CHESS_BOT_SEED"))
	if err != nil {
		return nil, err
	}
	randomize, err := launcherPromptBool(reader, output, "Vary near-best moves", envBool("CHESS_BOT_RANDOM", true))
	if err != nil {
		return nil, err
	}
	clock, err := launcherPrompt(reader, output, "Clock (blank disables)", os.Getenv("CHESS_CLOCK"))
	if err != nil {
		return nil, err
	}
	increment, err := launcherPrompt(reader, output, "Increment", os.Getenv("CHESS_INCREMENT"))
	if err != nil {
		return nil, err
	}
	theme, err := launcherPromptChoice(reader, output, "Theme", firstSet(os.Getenv("CHESS_THEME"), "unicode"), "ascii", "unicode")
	if err != nil {
		return nil, err
	}
	args := []string{"play", "bot"}
	if level != "" {
		args = launcherAppend(args, "--level", level)
	} else {
		args = launcherAppend(args, "--depth", depth)
	}
	args = launcherAppend(args, "--color", color)
	args = launcherAppend(args, "--personality", personality)
	args = launcherAppend(args, "--seed", seed)
	args = append(args, "--random="+strconv.FormatBool(randomize))
	args = launcherAppend(args, "--clock", clock)
	args = launcherAppend(args, "--increment", increment)
	args = launcherAppend(args, "--theme", theme)
	return args, nil
}

func launcherRemote(reader *bufio.Reader, output io.Writer) ([]string, error) {
	address, err := launcherPrompt(reader, output, "Server address", firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"))
	if err != nil {
		return nil, err
	}
	match, err := launcherPrompt(reader, output, "Match ID", os.Getenv("CHESS_MATCH_ID"))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(match) == "" {
		return nil, errors.New("a match ID is required")
	}
	player, err := launcherPrompt(reader, output, "Player ID", firstSet(os.Getenv("CHESS_PLAYER_ID"), os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER")))
	if err != nil {
		return nil, err
	}
	color, err := launcherPromptChoice(reader, output, "Color", firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white"), "white", "black", "spectator")
	if err != nil {
		return nil, err
	}
	token, err := launcherPromptSecret(reader, output, "Bearer token", os.Getenv("CHESS_NETWORK_TOKEN"))
	if err != nil {
		return nil, err
	}
	create, err := launcherPromptBool(reader, output, "Create if missing", false)
	if err != nil {
		return nil, err
	}
	clock, err := launcherPromptInt(reader, output, "Clock milliseconds", "0", 0)
	if err != nil {
		return nil, err
	}
	increment, err := launcherPromptInt(reader, output, "Increment milliseconds", "0", 0)
	if err != nil {
		return nil, err
	}
	theme, err := launcherPromptChoice(reader, output, "Theme", firstSet(os.Getenv("CHESS_THEME"), "unicode"), "ascii", "unicode")
	if err != nil {
		return nil, err
	}
	args := []string{"play", "remote", address, "--match", match}
	args = launcherAppend(args, "--player", player)
	args = launcherAppend(args, "--color", color)
	args = launcherAppend(args, "--token", token)
	if create {
		args = append(args, "--create")
	}
	args = launcherAppend(args, "--clock-millis", clock)
	args = launcherAppend(args, "--increment-millis", increment)
	args = launcherAppend(args, "--theme", theme)
	return args, nil
}

func launcherHost(reader *bufio.Reader, output io.Writer) ([]string, error) {
	address, err := launcherPrompt(reader, output, "Listen address", firstSet(os.Getenv("CHESS_NETWORK_ADDR"), ":8080"))
	if err != nil {
		return nil, err
	}
	token, err := launcherPromptSecret(reader, output, "Bearer token", os.Getenv("CHESS_NETWORK_TOKEN"))
	if err != nil {
		return nil, err
	}
	certificate, err := launcherPromptOptional(reader, output, "TLS certificate path", os.Getenv("CHESS_TLS_CERT"))
	if err != nil {
		return nil, err
	}
	key, err := launcherPromptOptional(reader, output, "TLS private key path", os.Getenv("CHESS_TLS_KEY"))
	if err != nil {
		return nil, err
	}
	insecure, err := launcherPromptBool(reader, output, "Allow insecure HTTP", envBool("CHESS_NETWORK_INSECURE", false))
	if err != nil {
		return nil, err
	}
	store, err := launcherPromptOptional(reader, output, "Match store path", os.Getenv("CHESS_MATCH_STORE"))
	if err != nil {
		return nil, err
	}
	lan, err := launcherPromptBool(reader, output, "Advertise on LAN", envBool("CHESS_LAN_DISCOVERY", false))
	if err != nil {
		return nil, err
	}
	instance, err := launcherPrompt(reader, output, "LAN instance", firstSet(os.Getenv("CHESS_LAN_INSTANCE"), "chess-go"))
	if err != nil {
		return nil, err
	}
	args := []string{"host", "--addr", address}
	args = launcherAppend(args, "--token", token)
	args = launcherAppend(args, "--cert", certificate)
	args = launcherAppend(args, "--key", key)
	if insecure {
		args = append(args, "--insecure")
	}
	args = launcherAppend(args, "--store", store)
	if lan {
		args = append(args, "--lan")
	}
	args = launcherAppend(args, "--lan-instance", instance)
	return args, nil
}

func launcherSeat(reader *bufio.Reader, output io.Writer, command string) ([]string, error) {
	address, err := launcherPrompt(reader, output, "Server address", firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"))
	if err != nil {
		return nil, err
	}
	match, err := launcherPrompt(reader, output, "Match ID", os.Getenv("CHESS_MATCH_ID"))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(match) == "" {
		return nil, errors.New("a match ID is required")
	}
	player, err := launcherPrompt(reader, output, "Player ID", firstSet(os.Getenv("CHESS_PLAYER_ID"), os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER")))
	if err != nil {
		return nil, err
	}
	colorDefault := firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white")
	if command == "spectate" {
		colorDefault = "spectator"
	}
	color, err := launcherPromptChoice(reader, output, "Color", colorDefault, "white", "black", "spectator")
	if err != nil {
		return nil, err
	}
	token, err := launcherPromptSecret(reader, output, "Bearer token", os.Getenv("CHESS_NETWORK_TOKEN"))
	if err != nil {
		return nil, err
	}
	args := []string{command, address, "--match", match}
	args = launcherAppend(args, "--player", player)
	if command == "spectate" {
		args = append(args, "--color", "spectator")
	} else {
		args = launcherAppend(args, "--color", color)
	}
	args = launcherAppend(args, "--token", token)
	return args, nil
}

func launcherMatchmake(reader *bufio.Reader, output io.Writer) ([]string, error) {
	address, err := launcherPrompt(reader, output, "Server address", firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"))
	if err != nil {
		return nil, err
	}
	player, err := launcherPrompt(reader, output, "Player ID", firstSet(os.Getenv("CHESS_PLAYER_ID"), os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER")))
	if err != nil {
		return nil, err
	}
	color, err := launcherPromptChoice(reader, output, "Preferred color", os.Getenv("CHESS_PLAYER_COLOR"), "", "white", "black", "random")
	if err != nil {
		return nil, err
	}
	token, err := launcherPromptSecret(reader, output, "Bearer token", os.Getenv("CHESS_NETWORK_TOKEN"))
	if err != nil {
		return nil, err
	}
	clock, err := launcherPromptInt(reader, output, "Clock milliseconds", "0", 0)
	if err != nil {
		return nil, err
	}
	increment, err := launcherPromptInt(reader, output, "Increment milliseconds", "0", 0)
	if err != nil {
		return nil, err
	}
	args := []string{"matchmake", address}
	args = launcherAppend(args, "--player", player)
	args = launcherAppend(args, "--color", color)
	args = launcherAppend(args, "--token", token)
	args = launcherAppend(args, "--clock-millis", clock)
	args = launcherAppend(args, "--increment-millis", increment)
	return args, nil
}

func launcherList(reader *bufio.Reader, output io.Writer) ([]string, error) {
	address, err := launcherPrompt(reader, output, "Server address", firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"))
	if err != nil {
		return nil, err
	}
	token, err := launcherPromptSecret(reader, output, "Bearer token", os.Getenv("CHESS_NETWORK_TOKEN"))
	if err != nil {
		return nil, err
	}
	args := []string{"list", address}
	args = launcherAppend(args, "--token", token)
	return args, nil
}

func launcherDiscover(reader *bufio.Reader, output io.Writer) ([]string, error) {
	seconds, err := launcherPromptInt(reader, output, "Discovery seconds", "2", 1)
	if err != nil {
		return nil, err
	}
	return []string{"discover", "--seconds", seconds}, nil
}

func launcherAppend(args []string, flag, value string) []string {
	if strings.TrimSpace(value) == "" {
		return args
	}
	return append(args, flag, value)
}

func launcherPromptOptional(reader *bufio.Reader, output io.Writer, label, defaultValue string) (string, error) {
	value, err := launcherPrompt(reader, output, label, defaultValue)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(strings.TrimSpace(value), "none") || strings.TrimSpace(value) == "-" {
		return "", nil
	}
	return value, nil
}

func launcherPromptChoice(reader *bufio.Reader, output io.Writer, label, defaultValue string, choices ...string) (string, error) {
	value, err := launcherPrompt(reader, output, label, defaultValue)
	if err != nil {
		return "", err
	}
	value = strings.ToLower(strings.TrimSpace(value))
	for _, choice := range choices {
		if value == strings.ToLower(choice) {
			return value, nil
		}
	}
	return "", fmt.Errorf("%s must be one of: %s", label, strings.Join(choices, ", "))
}

func launcherPromptInt(reader *bufio.Reader, output io.Writer, label, defaultValue string, minimum int64) (string, error) {
	value, err := launcherPrompt(reader, output, label, defaultValue)
	if err != nil {
		return "", err
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < minimum {
		return "", fmt.Errorf("%s must be an integer >= %d", label, minimum)
	}
	return strconv.FormatInt(parsed, 10), nil
}

func launcherPromptBool(reader *bufio.Reader, output io.Writer, label string, defaultValue bool) (bool, error) {
	value, err := launcherPrompt(reader, output, label+" (yes/no)", strconv.FormatBool(defaultValue))
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, nil
	case "0", "false", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be yes or no", label)
	}
}
