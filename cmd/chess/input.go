package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"chess-go"
)

type key uint8

const (
	keyUnknown key = iota
	keyUp
	keyDown
	keyLeft
	keyRight
	keySelect
	keyQuit
	keyUndo
	keyRedo
	keyNew
	keyHelp
	keyCommand
	keyEscape
)

func moveCursor(square chess.Square, pressed key, flipped bool) chess.Square {
	file, rank := int(square)%8, int(square)/8
	dFile, dRank := 0, 0
	switch pressed {
	case keyUp:
		dRank = 1
	case keyDown:
		dRank = -1
	case keyLeft:
		dFile = -1
	case keyRight:
		dFile = 1
	}
	if flipped {
		dFile, dRank = -dFile, -dRank
	}
	file, rank = max(0, min(7, file+dFile)), max(0, min(7, rank+dRank))
	return chess.Square(rank*8 + file)
}

func readKey(reader *bufio.Reader) (key, error) {
	char, err := reader.ReadByte()
	if err != nil {
		return keyUnknown, err
	}
	switch char {
	case '\r', '\n', ' ':
		return keySelect, nil
	case 'q', 3:
		return keyQuit, nil
	case 'u':
		return keyUndo, nil
	case 'r':
		return keyRedo, nil
	case 'n':
		return keyNew, nil
	case '?':
		return keyHelp, nil
	case ':':
		return keyCommand, nil
	case 'h':
		return keyLeft, nil
	case 'j':
		return keyDown, nil
	case 'k':
		return keyUp, nil
	case 'l':
		return keyRight, nil
	case 27:
		first, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return keyEscape, nil
			}
			return keyUnknown, err
		}
		if first != '[' {
			if unreadErr := reader.UnreadByte(); unreadErr != nil {
				return keyUnknown, unreadErr
			}
			return keyEscape, nil
		}
		second, err := reader.ReadByte()
		if err != nil {
			return keyUnknown, err
		}
		return map[byte]key{'A': keyUp, 'B': keyDown, 'C': keyRight, 'D': keyLeft}[second], nil
	default:
		return keyUnknown, nil
	}
}

func readRawLine(reader *bufio.Reader, output io.Writer) (string, error) {
	line := make([]byte, 0, 32)
	for {
		char, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		switch {
		case char == '\r' || char == '\n':
			fmt.Fprintln(output)
			return string(line), nil
		case (char == 8 || char == 127) && len(line) > 0:
			line = line[:len(line)-1]
			fmt.Fprint(output, "\b \b")
		case char >= 32 && char <= 126:
			line = append(line, char)
			fmt.Fprintf(output, "%c", char)
		}
	}
}

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
