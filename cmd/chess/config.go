package main

import (
	"os"
	"strconv"
)

// LocalConfig is the common configuration for a local terminal game.
type LocalConfig struct {
	Clock     string
	Increment string
	Theme     string
}

func localConfigFromEnv() LocalConfig {
	return LocalConfig{Clock: os.Getenv("CHESS_CLOCK"), Increment: os.Getenv("CHESS_INCREMENT"), Theme: firstSet(os.Getenv("CHESS_THEME"), "unicode")}
}

func (config LocalConfig) args() []string {
	args := []string{"play", "local"}
	args = launcherAppend(args, "--clock", config.Clock)
	args = launcherAppend(args, "--increment", config.Increment)
	return launcherAppend(args, "--theme", config.Theme)
}

// BotConfig is shared by environment defaults, CLI flags, and the launcher.
type BotConfig struct {
	Level       string
	Depth       int
	Color       string
	Personality string
	Seed        string
	Random      bool
	Clock       string
	Increment   string
	Theme       string
}

func botConfigFromEnv() (BotConfig, error) {
	depth, err := envInt("CHESS_BOT_DEPTH", 3)
	if err != nil {
		return BotConfig{}, err
	}
	return BotConfig{
		Level:       os.Getenv("CHESS_BOT_LEVEL"),
		Depth:       depth,
		Color:       firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white"),
		Personality: os.Getenv("CHESS_BOT_PERSONALITY"),
		Seed:        os.Getenv("CHESS_BOT_SEED"),
		Random:      envBool("CHESS_BOT_RANDOM", true),
		Clock:       os.Getenv("CHESS_CLOCK"),
		Increment:   os.Getenv("CHESS_INCREMENT"),
		Theme:       firstSet(os.Getenv("CHESS_THEME"), "unicode"),
	}, nil
}

func (config BotConfig) args() []string {
	args := []string{"play", "bot"}
	if config.Level != "" {
		args = launcherAppend(args, "--level", config.Level)
	} else {
		args = launcherAppend(args, "--depth", strconv.Itoa(config.Depth))
	}
	args = launcherAppend(args, "--color", config.Color)
	args = launcherAppend(args, "--personality", config.Personality)
	args = launcherAppend(args, "--seed", config.Seed)
	args = append(args, "--random="+strconv.FormatBool(config.Random))
	args = launcherAppend(args, "--clock", config.Clock)
	args = launcherAppend(args, "--increment", config.Increment)
	return launcherAppend(args, "--theme", config.Theme)
}

// RemoteConfig describes an interactive network game.
type RemoteConfig struct {
	Address         string
	Match           string
	Player          string
	Color           string
	Token           string
	Create          bool
	ClockMillis     int64
	IncrementMillis int64
	Theme           string
}

func remoteConfigFromEnv() RemoteConfig {
	return RemoteConfig{
		Address: firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"),
		Match:   os.Getenv("CHESS_MATCH_ID"),
		Player:  firstSet(os.Getenv("CHESS_PLAYER_ID"), os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER")),
		Color:   firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white"),
		Token:   os.Getenv("CHESS_NETWORK_TOKEN"),
		Theme:   firstSet(os.Getenv("CHESS_THEME"), "unicode"),
	}
}

func (config RemoteConfig) args() []string {
	args := []string{"play", "remote", config.Address, "--match", config.Match}
	args = launcherAppend(args, "--player", config.Player)
	args = launcherAppend(args, "--color", config.Color)
	args = launcherAppend(args, "--token", config.Token)
	if config.Create {
		args = append(args, "--create")
	}
	args = launcherAppend(args, "--clock-millis", strconv.FormatInt(config.ClockMillis, 10))
	args = launcherAppend(args, "--increment-millis", strconv.FormatInt(config.IncrementMillis, 10))
	return launcherAppend(args, "--theme", config.Theme)
}

// HostConfig contains all server and LAN-advertising options.
type HostConfig struct {
	Address     string
	Token       string
	Certificate string
	Key         string
	Insecure    bool
	Store       string
	LAN         bool
	LANInstance string
	LANHost     string
}

func hostConfigFromEnv() HostConfig {
	return HostConfig{
		Address:     firstSet(os.Getenv("CHESS_NETWORK_ADDR"), ":8080"),
		Token:       os.Getenv("CHESS_NETWORK_TOKEN"),
		Certificate: os.Getenv("CHESS_TLS_CERT"),
		Key:         os.Getenv("CHESS_TLS_KEY"),
		Insecure:    envBool("CHESS_NETWORK_INSECURE", false),
		Store:       os.Getenv("CHESS_MATCH_STORE"),
		LAN:         envBool("CHESS_LAN_DISCOVERY", false),
		LANInstance: firstSet(os.Getenv("CHESS_LAN_INSTANCE"), "chess-go"),
		LANHost:     os.Getenv("CHESS_LAN_HOST"),
	}
}

func (config HostConfig) args() []string {
	args := []string{"host", "--addr", config.Address}
	args = launcherAppend(args, "--token", config.Token)
	args = launcherAppend(args, "--cert", config.Certificate)
	args = launcherAppend(args, "--key", config.Key)
	if config.Insecure {
		args = append(args, "--insecure")
	}
	args = launcherAppend(args, "--store", config.Store)
	if config.LAN {
		args = append(args, "--lan")
	}
	return launcherAppend(args, "--lan-instance", config.LANInstance)
}

// SeatConfig is used by join, connect, and spectate.
type SeatConfig struct {
	Command string
	Address string
	Match   string
	Player  string
	Color   string
	Token   string
}

func seatConfigFromEnv(command string) SeatConfig {
	color := firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white")
	if command == "spectate" {
		color = "spectator"
	}
	return SeatConfig{
		Command: command,
		Address: firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"),
		Match:   os.Getenv("CHESS_MATCH_ID"),
		Player:  firstSet(os.Getenv("CHESS_PLAYER_ID"), os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER")),
		Color:   color,
		Token:   os.Getenv("CHESS_NETWORK_TOKEN"),
	}
}

func (config SeatConfig) args() []string {
	args := []string{config.Command, config.Address, "--match", config.Match}
	args = launcherAppend(args, "--player", config.Player)
	if config.Command == "spectate" {
		args = append(args, "--color", "spectator")
	} else {
		args = launcherAppend(args, "--color", config.Color)
	}
	return launcherAppend(args, "--token", config.Token)
}

// MatchmakeConfig describes a matchmaking request.
type MatchmakeConfig struct {
	Address         string
	Player          string
	Color           string
	Token           string
	ClockMillis     int64
	IncrementMillis int64
}

func matchmakeConfigFromEnv() MatchmakeConfig {
	return MatchmakeConfig{
		Address: firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"),
		Player:  firstSet(os.Getenv("CHESS_PLAYER_ID"), os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER")),
		Color:   os.Getenv("CHESS_PLAYER_COLOR"),
		Token:   os.Getenv("CHESS_NETWORK_TOKEN"),
	}
}

func (config MatchmakeConfig) args() []string {
	args := []string{"matchmake", config.Address}
	args = launcherAppend(args, "--player", config.Player)
	args = launcherAppend(args, "--color", config.Color)
	args = launcherAppend(args, "--token", config.Token)
	args = launcherAppend(args, "--clock-millis", strconv.FormatInt(config.ClockMillis, 10))
	return launcherAppend(args, "--increment-millis", strconv.FormatInt(config.IncrementMillis, 10))
}

// ListConfig and DiscoverConfig keep the remaining network commands typed too.
type ListConfig struct {
	Address string
	Token   string
}

func listConfigFromEnv() ListConfig {
	return ListConfig{Address: firstSet(os.Getenv("CHESS_NETWORK_URL"), "https://127.0.0.1:8080"), Token: os.Getenv("CHESS_NETWORK_TOKEN")}
}

func (config ListConfig) args() []string {
	return launcherAppend([]string{"list", config.Address}, "--token", config.Token)
}

type DiscoverConfig struct {
	Seconds int
}

func (config DiscoverConfig) args() []string {
	return []string{"discover", "--seconds", strconv.Itoa(config.Seconds)}
}
