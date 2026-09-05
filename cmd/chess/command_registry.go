package main

import (
	"fmt"
	"strings"
)

// commandSpec is the single user-facing command description used by top-level
// help and the interactive launcher. MenuAction is either the command name,
// "network" for the network submenu, or empty for commands with no launcher
// form.
type commandSpec struct {
	Name           string
	Usage          string
	MenuAction     string
	MenuLabel      string
	MenuHint       string
	MenuGroupLabel string
	MenuGroupHint  string
}

var commandRegistry = []commandSpec{
	{Name: "play local", Usage: "chess play local [options]", MenuAction: "local", MenuLabel: "Local game", MenuHint: "play standard chess locally"},
	{Name: "play bot", Usage: "chess play bot [options]", MenuAction: "bot", MenuLabel: "Play against bot", MenuHint: "choose strength, personality, and variation"},
	{Name: "play remote", Usage: "chess play remote ADDRESS [options]", MenuAction: "remote", MenuLabel: "Remote game", MenuHint: "create or join an online match"},
	{Name: "host", Usage: "chess host [options]", MenuAction: "network", MenuLabel: "Host server", MenuHint: "serve an encrypted match endpoint", MenuGroupLabel: "Network tools", MenuGroupHint: "host, join, spectate, matchmake, list, discover"},
	{Name: "join", Usage: "chess join ADDRESS [options]", MenuAction: "network", MenuLabel: "Join match", MenuHint: "claim a player seat", MenuGroupLabel: "Network tools", MenuGroupHint: "host, join, spectate, matchmake, list, discover"},
	{Name: "connect", Usage: "chess connect ADDRESS [options]", MenuAction: "network", MenuLabel: "Connect", MenuHint: "join alias", MenuGroupLabel: "Network tools", MenuGroupHint: "host, join, spectate, matchmake, list, discover"},
	{Name: "spectate", Usage: "chess spectate ADDRESS [options]", MenuAction: "network", MenuLabel: "Spectate match", MenuHint: "watch without a seat", MenuGroupLabel: "Network tools", MenuGroupHint: "host, join, spectate, matchmake, list, discover"},
	{Name: "matchmake", Usage: "chess matchmake ADDRESS [options]", MenuAction: "network", MenuLabel: "Matchmake", MenuHint: "find or create an open match", MenuGroupLabel: "Network tools", MenuGroupHint: "host, join, spectate, matchmake, list, discover"},
	{Name: "list", Usage: "chess list ADDRESS [options]", MenuAction: "network", MenuLabel: "List matches", MenuHint: "show available matches", MenuGroupLabel: "Network tools", MenuGroupHint: "host, join, spectate, matchmake, list, discover"},
	{Name: "discover", Usage: "chess discover [options]", MenuAction: "network", MenuLabel: "Discover LAN", MenuHint: "find advertised hosts", MenuGroupLabel: "Network tools", MenuGroupHint: "host, join, spectate, matchmake, list, discover"},
	{Name: "load", Usage: "chess load FILE", MenuAction: "load", MenuLabel: "Load PGN", MenuHint: "open a saved game"},
	{Name: "help", Usage: "chess help", MenuAction: "help", MenuLabel: "Help", MenuHint: "keyboard and command reference"},
	{Name: "version", Usage: "chess version", MenuAction: "version", MenuLabel: "Version", MenuHint: "show the installed chess-go version"},
}

type launcherItem struct {
	label  string
	hint   string
	action string
}

var launcherItems = buildLauncherItems()

func buildLauncherItems() []launcherItem {
	items := make([]launcherItem, 0, 9)
	for _, action := range []string{"local", "bot", "remote", "network", "load"} {
		if item, ok := launcherItemFor(action); ok {
			items = append(items, item)
		}
	}
	items = append(items,
		launcherItem{label: "Settings", hint: "theme, pieces, bot variation, and network security", action: "settings"},
	)
	for _, action := range []string{"help", "version"} {
		if item, ok := launcherItemFor(action); ok {
			items = append(items, item)
		}
	}
	items = append(items, launcherItem{label: "Quit", hint: "leave the launcher", action: "quit"})
	return items
}

func launcherItemFor(action string) (launcherItem, bool) {
	for _, spec := range commandRegistry {
		if spec.MenuAction != action {
			continue
		}
		if action == "network" {
			return launcherItem{label: spec.MenuGroupLabel, hint: spec.MenuGroupHint, action: action}, true
		}
		return launcherItem{label: spec.MenuLabel, hint: spec.MenuHint, action: action}, true
	}
	return launcherItem{}, false
}

func networkLauncherItems() []launcherItem {
	items := make([]launcherItem, 0, 8)
	for _, spec := range commandRegistry {
		if spec.MenuAction == "network" {
			items = append(items, launcherItem{label: spec.MenuLabel, hint: spec.MenuHint, action: spec.Name})
		}
	}
	return append(items, launcherItem{label: "Back", hint: "return to the launcher", action: "back"})
}

func commandUsageSummary() string {
	usages := make([]string, 0, len(commandRegistry))
	for _, spec := range commandRegistry {
		usages = append(usages, spec.Usage)
	}
	return fmt.Sprintf("Usage: chess [menu] | %s", strings.Join(usages, " | "))
}
