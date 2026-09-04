package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"chess-go"
	"chess-go/lan"
	"chess-go/protocol"
	"chess-go/storage"
	"chess-go/transport"
)

const remoteSpectator chess.Color = 255

func runHost(ctx context.Context, args []string, output io.Writer) error {
	options := flag.NewFlagSet("host", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	address := options.String("addr", firstSet(os.Getenv("CHESS_NETWORK_ADDR"), ":8080"), "listen address")
	token := options.String("token", os.Getenv("CHESS_NETWORK_TOKEN"), "optional bearer token")
	certificate := options.String("cert", os.Getenv("CHESS_TLS_CERT"), "TLS certificate PEM path")
	key := options.String("key", os.Getenv("CHESS_TLS_KEY"), "TLS private key PEM path")
	storePath := options.String("store", os.Getenv("CHESS_MATCH_STORE"), "durable match state JSON path")
	lanEnabled := options.Bool("lan", envBool("CHESS_LAN_DISCOVERY", false), "advertise this host with DNS-SD/mDNS")
	instance := options.String("lan-instance", firstSet(os.Getenv("CHESS_LAN_INSTANCE"), "chess-go"), "LAN service instance name")
	if err := options.Parse(args); err != nil || options.NArg() != 0 {
		return errors.New("usage: chess host [--addr ADDRESS] [--token TOKEN] [--cert FILE --key FILE] [--store FILE] [--lan]")
	}
	if (*certificate == "") != (*key == "") {
		return errors.New("--cert and --key must be provided together")
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return err
	}
	defer listener.Close()
	authority := protocol.NewServer()
	var store *storage.FileStore
	if *storePath != "" {
		store, err = storage.NewFileStore(*storePath)
		if err != nil {
			return err
		}
		if err := store.Load(authority); err != nil && !os.IsNotExist(err) {
			return err
		}
		authority.SetPersistenceHook(store.SaveStates)
		defer func() { _ = store.SaveServer(authority) }()
	}
	var advertiser *lan.Advertiser
	if *lanEnabled {
		host := os.Getenv("CHESS_LAN_HOST")
		if host == "" {
			host, err = os.Hostname()
			if err != nil {
				return err
			}
		}
		tcpAddress, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			return errors.New("LAN discovery requires a TCP listener")
		}
		advertiser, err = lan.NewAdvertiser(lan.Service{Instance: *instance, Host: host, Port: tcpAddress.Port, Metadata: map[string]string{"protocol": "1"}})
		if err != nil {
			return err
		}
		defer advertiser.Close()
		go func() {
			if err := advertiser.Serve(ctx); err != nil && ctx.Err() == nil {
				fmt.Fprintf(output, "LAN discovery: %v\n", err)
			}
		}()
	}
	mux := http.NewServeMux()
	mux.Handle("/ws", transport.NewWebSocketServer(authority, *token))
	mux.Handle("/", transport.NewHTTPServer(authority, *token))
	httpServer := &http.Server{Handler: mux}
	serveDone := make(chan error, 1)
	go func() {
		if *certificate != "" {
			serveDone <- httpServer.ServeTLS(listener, *certificate, *key)
			return
		}
		serveDone <- httpServer.Serve(listener)
	}()
	fmt.Fprintf(output, "Hosting chess server on %s\n", listener.Addr())
	select {
	case err := <-serveDone:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownContext)
		return ctx.Err()
	}
}

func runDiscover(ctx context.Context, args []string, output io.Writer) error {
	options := flag.NewFlagSet("discover", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	seconds := options.Int("seconds", 2, "discovery duration")
	if err := options.Parse(args); err != nil || options.NArg() != 0 || *seconds < 1 {
		return errors.New("usage: chess discover [--seconds N]")
	}
	discoveryContext, cancel := context.WithTimeout(ctx, time.Duration(*seconds)*time.Second)
	defer cancel()
	services, err := lan.Discover(discoveryContext)
	if err != nil {
		return err
	}
	for _, service := range services {
		fmt.Fprintf(output, "%s http://%s:%d", service.Instance, service.Host, service.Port)
		if len(service.Metadata) != 0 {
			fmt.Fprintf(output, " %#v", service.Metadata)
		}
		fmt.Fprintln(output)
	}
	return nil
}

func runNetworkCommand(ctx context.Context, command string, args []string, output io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: chess list ADDRESS | chess join|connect|spectate ADDRESS --match ID [options]")
	}
	address := args[0]
	if command == "list" {
		options := flag.NewFlagSet("list", flag.ContinueOnError)
		options.SetOutput(io.Discard)
		token := options.String("token", os.Getenv("CHESS_NETWORK_TOKEN"), "bearer token")
		if err := options.Parse(args[1:]); err != nil || options.NArg() != 0 {
			return errors.New("usage: chess list ADDRESS [--token TOKEN]")
		}
		client, err := transport.NewClient(address, *token)
		if err != nil {
			return err
		}
		matches, err := client.List(ctx)
		if err != nil {
			return err
		}
		for _, match := range matches {
			printNetworkSnapshot(output, match)
		}
		return nil
	}
	options := flag.NewFlagSet(command, flag.ContinueOnError)
	options.SetOutput(io.Discard)
	matchID := options.String("match", os.Getenv("CHESS_MATCH_ID"), "match ID")
	playerID := options.String("player", firstSet(os.Getenv("CHESS_PLAYER_ID"), os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER")), "player ID")
	color := options.String("color", firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white"), "white, black, or spectator")
	token := options.String("token", os.Getenv("CHESS_NETWORK_TOKEN"), "bearer token")
	if command == "spectate" {
		*color = "spectator"
	}
	if err := options.Parse(args[1:]); err != nil || options.NArg() != 0 {
		return fmt.Errorf("usage: chess %s ADDRESS --match ID [--player ID] [--color white|black|spectator]", command)
	}
	if strings.TrimSpace(*matchID) == "" {
		return errors.New("--match is required")
	}
	client, err := transport.NewClient(address, *token)
	if err != nil {
		return err
	}
	snapshot, err := client.Join(ctx, command+"-join", protocol.JoinMatchRequest{MatchID: *matchID, PlayerID: *playerID, Color: *color})
	if err != nil {
		return err
	}
	printNetworkSnapshot(output, snapshot)
	return nil
}

func runRemote(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: chess play remote ADDRESS --match ID [options]")
	}
	address := args[0]
	options := flag.NewFlagSet("play remote", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	matchID := options.String("match", os.Getenv("CHESS_MATCH_ID"), "match ID")
	playerID := options.String("player", firstSet(os.Getenv("CHESS_PLAYER_ID"), os.Getenv("CHESS_PLAYER_NAME"), os.Getenv("USER")), "player ID")
	color := options.String("color", firstSet(os.Getenv("CHESS_PLAYER_COLOR"), "white"), "white, black, or spectator")
	token := options.String("token", os.Getenv("CHESS_NETWORK_TOKEN"), "bearer token")
	create := options.Bool("create", false, "create the match if it does not exist")
	clockMillis := options.Int64("clock-millis", 0, "initial time per side in milliseconds")
	incrementMillis := options.Int64("increment-millis", 0, "increment per move in milliseconds")
	themeName := options.String("theme", firstSet(os.Getenv("CHESS_THEME"), "ascii"), "board theme")
	if err := options.Parse(args[1:]); err != nil || options.NArg() != 0 {
		return errors.New("usage: chess play remote ADDRESS --match ID [--player ID] [--color white|black|spectator]")
	}
	if strings.TrimSpace(*matchID) == "" {
		return errors.New("--match is required")
	}
	boardTheme, err := parseTheme(*themeName)
	if err != nil {
		return err
	}
	remoteColor, err := parseRemoteColor(*color)
	if err != nil {
		return err
	}
	client, err := transport.NewClient(address, *token)
	if err != nil {
		return err
	}
	var snapshot protocol.MatchSnapshot
	if *create {
		snapshot, err = client.Create(ctx, "remote-create", protocol.CreateMatchRequest{MatchID: *matchID, PlayerID: *playerID, Color: *color, ClockMillis: *clockMillis, IncrementMillis: *incrementMillis})
	} else {
		snapshot, err = client.Join(ctx, "remote-join", protocol.JoinMatchRequest{MatchID: *matchID, PlayerID: *playerID, Color: *color})
	}
	if err != nil {
		return err
	}
	position, err := chess.ParseFEN(snapshot.FEN)
	if err != nil {
		return err
	}
	game := chess.NewGameFromPosition(position)
	scanner := bufio.NewScanner(input)
	fmt.Fprintln(output, "Remote commands: UCI move, refresh, draw, resign, quit")
	for {
		if err := renderRemote(output, game, snapshot, remoteColor, boardTheme); err != nil {
			return err
		}
		if snapshot.Result != "*" {
			fmt.Fprintln(output, "Game over:", snapshot.Result)
			return nil
		}
		fmt.Fprint(output, "remote > ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		switch strings.ToLower(line) {
		case "", "help":
			continue
		case "quit", "exit":
			return nil
		case "refresh", "sync":
			snapshot, err = client.Snapshot(ctx, protocol.SnapshotRequest{MatchID: *matchID, PlayerID: *playerID})
			if err == nil {
				position, err = chess.ParseFEN(snapshot.FEN)
				if err == nil {
					game = chess.NewGameFromPosition(position)
				}
			}
			if err != nil {
				fmt.Fprintln(output, "Error:", err)
			}
			continue
		case "draw":
			snapshot, err = client.OfferDraw(ctx, "remote-draw", protocol.DrawOfferRequest{MatchID: *matchID, PlayerID: *playerID})
		case "resign":
			snapshot, err = client.Resign(ctx, "remote-resign", protocol.ResignRequest{MatchID: *matchID, PlayerID: *playerID})
		default:
			if remoteColor == remoteSpectator {
				fmt.Fprintln(output, "Error: spectators cannot move")
				continue
			}
			if strings.ToLower(snapshot.Turn) != colorNameLower(remoteColor) {
				fmt.Fprintln(output, "Error: waiting for the opponent")
				continue
			}
			accepted, moveErr := client.Move(ctx, "remote-move", protocol.MoveRequest{MatchID: *matchID, PlayerID: *playerID, Sequence: snapshot.Sequence, PositionHash: snapshot.PositionHash, UCI: line})
			if moveErr != nil {
				fmt.Fprintln(output, "Error:", moveErr)
				continue
			}
			snapshot, err = client.Snapshot(ctx, protocol.SnapshotRequest{MatchID: *matchID, PlayerID: *playerID})
			if err != nil {
				// The acknowledgement is authoritative even if a follow-up read fails.
				snapshot.Sequence, snapshot.PositionHash, snapshot.FEN, snapshot.Result = accepted.Sequence, accepted.PositionHash, accepted.FEN, accepted.Result
				err = nil
			}
		}
		if err != nil {
			fmt.Fprintln(output, "Error:", err)
			continue
		}
		position, err = chess.ParseFEN(snapshot.FEN)
		if err != nil {
			return err
		}
		game = chess.NewGameFromPosition(position)
	}
}

func renderRemote(output io.Writer, game *chess.Game, snapshot protocol.MatchSnapshot, color chess.Color, boardTheme theme) error {
	clock := ""
	if snapshot.IncrementMillis != 0 || snapshot.WhiteTimeMillis != 0 || snapshot.BlackTimeMillis != 0 {
		clock = fmt.Sprintf("White %s · Black %s · +%s", formatClock(time.Duration(snapshot.WhiteTimeMillis)*time.Millisecond), formatClock(time.Duration(snapshot.BlackTimeMillis)*time.Millisecond), formatClock(time.Duration(snapshot.IncrementMillis)*time.Millisecond))
	}
	render(output, game, color == chess.Black, clock, boardTheme)
	return nil
}

func printNetworkSnapshot(output io.Writer, snapshot protocol.MatchSnapshot) {
	fmt.Fprintf(output, "Match %s · sequence %d · %s to move · result %s\n", snapshot.MatchID, snapshot.Sequence, snapshot.Turn, snapshot.Result)
	fmt.Fprintln(output, snapshot.FEN)
	if snapshot.ClockRunning || snapshot.WhiteTimeMillis != 0 || snapshot.BlackTimeMillis != 0 {
		fmt.Fprintf(output, "White %dms · Black %dms · +%dms\n", snapshot.WhiteTimeMillis, snapshot.BlackTimeMillis, snapshot.IncrementMillis)
	}
}

func parseRemoteColor(value string) (chess.Color, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "white":
		return chess.White, nil
	case "black":
		return chess.Black, nil
	case "spectator", "watcher":
		return remoteSpectator, nil
	default:
		return chess.White, fmt.Errorf("invalid color %q (choose white, black, or spectator)", value)
	}
}

func colorNameLower(color chess.Color) string {
	if color == chess.White {
		return "white"
	}
	return "black"
}

func envBool(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
