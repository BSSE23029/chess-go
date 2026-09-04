package main

import (
	"fmt"
	"strings"

	"chess-go"
)

type theme struct {
	name   string
	pieces [2][7]rune
}

var asciiTheme = theme{name: "ascii", pieces: [2][7]rune{
	{' ', 'P', 'N', 'B', 'R', 'Q', 'K'},
	{' ', 'p', 'n', 'b', 'r', 'q', 'k'},
}}

var unicodeTheme = theme{name: "unicode", pieces: [2][7]rune{
	{' ', '♙', '♘', '♗', '♖', '♕', '♔'},
	{' ', '♟', '♞', '♝', '♜', '♛', '♚'},
}}

func parseTheme(value string) (theme, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "ascii", "letters":
		return asciiTheme, nil
	case "unicode", "symbols":
		return unicodeTheme, nil
	default:
		return theme{}, fmt.Errorf("unknown theme %q (choose ascii or unicode)", value)
	}
}

func (t theme) glyph(piece chess.Piece) rune {
	if piece.IsEmpty() {
		return '.'
	}
	color := int(piece.Color)
	if color < 0 || color > 1 || piece.Type > chess.King {
		return '?'
	}
	return t.pieces[color][piece.Type]
}

func (t theme) label() string {
	if t.name == "" {
		return asciiTheme.name
	}
	return t.name
}
