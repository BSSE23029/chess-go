package main

import (
	"strings"
	"unicode/utf8"
)

func formatInteractiveFrame(frame string, width, height int) string {
	const clear = "\x1b[H\x1b[2J"
	prefix := clear
	if strings.HasPrefix(frame, tuiFrameStart) {
		prefix = tuiFrameStart
	}
	if !strings.HasPrefix(frame, prefix) {
		return frame
	}
	body := strings.TrimSuffix(strings.TrimPrefix(frame, prefix), "\n")
	lines := strings.Split(body, "\n")
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	maxWidth := 0
	for _, line := range lines {
		if current := len([]rune(stripSGR(line))); current > maxWidth {
			maxWidth = current
		}
	}
	margin := 0
	if width > maxWidth {
		margin = (width - maxWidth) / 2
	}
	if margin > 0 {
		prefix := strings.Repeat(" ", margin)
		for index, line := range lines {
			lines[index] = prefix + line
		}
	}
	if width > 0 {
		for index, line := range lines {
			lines[index] = truncateTerminalLine(line, width)
		}
	}
	// Erasing a screen does not consistently paint the terminal's configured
	// background (notably in macOS Terminal). Paint every viewport row and
	// column explicitly so a light profile cannot leak through around the UI.
	if height > 0 {
		for len(lines) < height {
			lines = append(lines, "")
		}
	}
	if width > 0 {
		for index, line := range lines {
			visible := len([]rune(stripSGR(line)))
			if visible < width {
				lines[index] = tuiSurface + line + strings.Repeat(" ", width-visible) + tuiReset
			}
		}
	}
	return prefix + strings.Join(lines, "\n")
}

func truncateTerminalLine(line string, width int) string {
	if width < 1 {
		return ""
	}
	visible := 0
	var out strings.Builder
	for index := 0; index < len(line); {
		if line[index] == 0x1b && index+1 < len(line) && line[index+1] == '[' {
			end := strings.IndexByte(line[index+2:], 'm')
			if end >= 0 {
				end += index + 2
				out.WriteString(line[index : end+1])
				index = end + 1
				continue
			}
		}
		if visible >= width {
			break
		}
		_, size := utf8.DecodeRuneInString(line[index:])
		if size == 0 {
			break
		}
		out.WriteString(line[index : index+size])
		index += size
		visible++
	}
	if visible < len([]rune(stripSGR(line))) {
		out.WriteString(tuiReset)
	}
	return out.String()
}

func railLine(rail []string, boardRow, cellHeight, boardRows int) string {
	if len(rail) == 0 {
		return ""
	}
	if boardRow == 0 {
		return rail[0]
	}
	if boardRow == boardRows-1 {
		return rail[len(rail)-1]
	}
	if cellHeight < 1 || (boardRow-1)%cellHeight != 0 {
		return ""
	}
	index := 1 + (boardRow-1)/cellHeight
	if index >= len(rail)-1 {
		return ""
	}
	return rail[index]
}
