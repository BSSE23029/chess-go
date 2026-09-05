package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func launcherLocal(reader *bufio.Reader, output io.Writer) ([]string, error) {
	config := localConfigFromEnv()
	var err error
	config.Clock, err = launcherPrompt(reader, output, "Clock (blank disables)", config.Clock)
	if err != nil {
		return nil, err
	}
	config.Increment, err = launcherPrompt(reader, output, "Increment", config.Increment)
	if err != nil {
		return nil, err
	}
	config.Theme, err = launcherPromptChoice(reader, output, "Theme", config.Theme, "ascii", "unicode")
	if err != nil {
		return nil, err
	}
	return config.args(), nil
}

func launcherBot(reader *bufio.Reader, output io.Writer) ([]string, error) {
	config, err := botConfigFromEnv()
	if err != nil {
		return nil, err
	}
	config.Level, err = launcherPromptOptional(reader, output, "Strength level (or - for depth)", config.Level)
	if err != nil {
		return nil, err
	}
	depth, err := launcherPromptInt(reader, output, "Search depth", strconv.Itoa(config.Depth), 1)
	if err != nil {
		return nil, err
	}
	config.Depth, err = strconv.Atoi(depth)
	if err != nil {
		return nil, err
	}
	config.Color, err = launcherPromptChoice(reader, output, "Human color", config.Color, "white", "black")
	if err != nil {
		return nil, err
	}
	config.Personality, err = launcherPromptOptional(reader, output, "Personality (optional)", config.Personality)
	if err != nil {
		return nil, err
	}
	config.Seed, err = launcherPromptOptional(reader, output, "Seed (optional)", config.Seed)
	if err != nil {
		return nil, err
	}
	config.Random, err = launcherPromptBool(reader, output, "Vary near-best moves", config.Random)
	if err != nil {
		return nil, err
	}
	config.Clock, err = launcherPrompt(reader, output, "Clock (blank disables)", config.Clock)
	if err != nil {
		return nil, err
	}
	config.Increment, err = launcherPrompt(reader, output, "Increment", config.Increment)
	if err != nil {
		return nil, err
	}
	config.Theme, err = launcherPromptChoice(reader, output, "Theme", config.Theme, "ascii", "unicode")
	if err != nil {
		return nil, err
	}
	return config.args(), nil
}

func launcherRemote(reader *bufio.Reader, output io.Writer) ([]string, error) {
	config := remoteConfigFromEnv()
	var err error
	config.Address, err = launcherPrompt(reader, output, "Server address", config.Address)
	if err != nil {
		return nil, err
	}
	config.Match, err = launcherPrompt(reader, output, "Match ID", config.Match)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(config.Match) == "" {
		return nil, errors.New("a match ID is required")
	}
	config.Player, err = launcherPrompt(reader, output, "Player ID", config.Player)
	if err != nil {
		return nil, err
	}
	config.Color, err = launcherPromptChoice(reader, output, "Color", config.Color, "white", "black", "spectator")
	if err != nil {
		return nil, err
	}
	config.Token, err = launcherPromptSecret(reader, output, "Bearer token", config.Token)
	if err != nil {
		return nil, err
	}
	config.Create, err = launcherPromptBool(reader, output, "Create if missing", false)
	if err != nil {
		return nil, err
	}
	clock, err := launcherPromptInt(reader, output, "Clock milliseconds", "0", 0)
	if err != nil {
		return nil, err
	}
	config.ClockMillis, err = strconv.ParseInt(clock, 10, 64)
	if err != nil {
		return nil, err
	}
	increment, err := launcherPromptInt(reader, output, "Increment milliseconds", "0", 0)
	if err != nil {
		return nil, err
	}
	config.IncrementMillis, err = strconv.ParseInt(increment, 10, 64)
	if err != nil {
		return nil, err
	}
	config.Theme, err = launcherPromptChoice(reader, output, "Theme", config.Theme, "ascii", "unicode")
	if err != nil {
		return nil, err
	}
	return config.args(), nil
}

func launcherHost(reader *bufio.Reader, output io.Writer) ([]string, error) {
	config := hostConfigFromEnv()
	var err error
	config.Address, err = launcherPrompt(reader, output, "Listen address", config.Address)
	if err != nil {
		return nil, err
	}
	config.Token, err = launcherPromptSecret(reader, output, "Bearer token", config.Token)
	if err != nil {
		return nil, err
	}
	config.Certificate, err = launcherPromptOptional(reader, output, "TLS certificate path", config.Certificate)
	if err != nil {
		return nil, err
	}
	config.Key, err = launcherPromptOptional(reader, output, "TLS private key path", config.Key)
	if err != nil {
		return nil, err
	}
	config.Insecure, err = launcherPromptBool(reader, output, "Allow insecure HTTP", config.Insecure)
	if err != nil {
		return nil, err
	}
	config.Store, err = launcherPromptOptional(reader, output, "Match store path", config.Store)
	if err != nil {
		return nil, err
	}
	config.LAN, err = launcherPromptBool(reader, output, "Advertise on LAN", config.LAN)
	if err != nil {
		return nil, err
	}
	config.LANInstance, err = launcherPrompt(reader, output, "LAN instance", config.LANInstance)
	if err != nil {
		return nil, err
	}
	config.LANHost, err = launcherPromptOptional(reader, output, "LAN advertised host (optional)", config.LANHost)
	if err != nil {
		return nil, err
	}
	setLauncherEnv("CHESS_LAN_HOST", config.LANHost)
	return config.args(), nil
}

func launcherSeat(reader *bufio.Reader, output io.Writer, command string) ([]string, error) {
	config := seatConfigFromEnv(command)
	var err error
	config.Address, err = launcherPrompt(reader, output, "Server address", config.Address)
	if err != nil {
		return nil, err
	}
	config.Match, err = launcherPrompt(reader, output, "Match ID", config.Match)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(config.Match) == "" {
		return nil, errors.New("a match ID is required")
	}
	config.Player, err = launcherPrompt(reader, output, "Player ID", config.Player)
	if err != nil {
		return nil, err
	}
	config.Color, err = launcherPromptChoice(reader, output, "Color", config.Color, "white", "black", "spectator")
	if err != nil {
		return nil, err
	}
	config.Token, err = launcherPromptSecret(reader, output, "Bearer token", config.Token)
	if err != nil {
		return nil, err
	}
	return config.args(), nil
}

func launcherMatchmake(reader *bufio.Reader, output io.Writer) ([]string, error) {
	config := matchmakeConfigFromEnv()
	var err error
	config.Address, err = launcherPrompt(reader, output, "Server address", config.Address)
	if err != nil {
		return nil, err
	}
	config.Player, err = launcherPrompt(reader, output, "Player ID", config.Player)
	if err != nil {
		return nil, err
	}
	config.Color, err = launcherPromptChoice(reader, output, "Preferred color", config.Color, "", "white", "black", "random")
	if err != nil {
		return nil, err
	}
	config.Token, err = launcherPromptSecret(reader, output, "Bearer token", config.Token)
	if err != nil {
		return nil, err
	}
	clock, err := launcherPromptInt(reader, output, "Clock milliseconds", "0", 0)
	if err != nil {
		return nil, err
	}
	config.ClockMillis, err = strconv.ParseInt(clock, 10, 64)
	if err != nil {
		return nil, err
	}
	increment, err := launcherPromptInt(reader, output, "Increment milliseconds", "0", 0)
	if err != nil {
		return nil, err
	}
	config.IncrementMillis, err = strconv.ParseInt(increment, 10, 64)
	if err != nil {
		return nil, err
	}
	return config.args(), nil
}

func launcherList(reader *bufio.Reader, output io.Writer) ([]string, error) {
	config := listConfigFromEnv()
	var err error
	config.Address, err = launcherPrompt(reader, output, "Server address", config.Address)
	if err != nil {
		return nil, err
	}
	config.Token, err = launcherPromptSecret(reader, output, "Bearer token", config.Token)
	if err != nil {
		return nil, err
	}
	return config.args(), nil
}

func launcherDiscover(reader *bufio.Reader, output io.Writer) ([]string, error) {
	seconds, err := launcherPromptInt(reader, output, "Discovery seconds", "2", 1)
	if err != nil {
		return nil, err
	}
	parsed, err := strconv.Atoi(seconds)
	if err != nil {
		return nil, err
	}
	return (DiscoverConfig{Seconds: parsed}).args(), nil
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
