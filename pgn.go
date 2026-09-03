package chess

import (
	"fmt"
	"strings"
)

// PGNTag is one metadata tag pair from a PGN game.
type PGNTag struct {
	// Name is the case-sensitive PGN tag name.
	Name string
	// Value is the decoded tag value.
	Value string
}

// Tags returns an independent copy of the game's PGN metadata in source order.
func (g *Game) Tags() []PGNTag {
	return append([]PGNTag(nil), g.tags...)
}

// PGN serializes the played main line and retained metadata.
func (g *Game) PGN() string {
	result := g.Result()
	values := make(map[string]string, len(g.tags))
	for _, tag := range g.tags {
		values[tag.Name] = tag.Value
	}
	outputTags := append([]PGNTag(nil), g.tags...)
	defaults := []PGNTag{{"Event", "?"}, {"Site", "?"}, {"Date", "????.??.??"}, {"Round", "?"}, {"White", "?"}, {"Black", "?"}, {"Result", result}}
	if g.positions[0].FEN() != InitialFEN {
		defaults = append(defaults, PGNTag{"SetUp", "1"}, PGNTag{"FEN", g.positions[0].FEN()})
	}
	for _, tag := range defaults {
		if _, exists := values[tag.Name]; !exists {
			outputTags = append(outputTags, tag)
		}
	}
	var output strings.Builder
	for _, tag := range outputTags {
		value := tag.Value
		switch tag.Name {
		case "Result":
			value = result
		case "SetUp":
			if g.positions[0].FEN() != InitialFEN {
				value = "1"
			}
		case "FEN":
			value = g.positions[0].FEN()
		}
		fmt.Fprintf(&output, "[%s \"%s\"]\n", tag.Name, escapePGNValue(value))
	}
	output.WriteByte('\n')
	for i := 0; i < g.cursor; i++ {
		position := g.positions[i]
		if i > 0 {
			output.WriteByte(' ')
		}
		if position.turn == White {
			fmt.Fprintf(&output, "%d. ", position.fullmoveNumber)
		} else if i == 0 {
			fmt.Fprintf(&output, "%d... ", position.fullmoveNumber)
		}
		san, _ := position.SAN(g.moves[i])
		output.WriteString(san)
	}
	if g.cursor > 0 {
		output.WriteByte(' ')
	}
	output.WriteString(result)
	return output.String()
}

// ParsePGN parses PGN metadata and its main-line moves.
func ParsePGN(value string) (*Game, error) {
	tags, movetext, err := splitPGN(value)
	if err != nil {
		return nil, err
	}
	lookup := make(map[string]string, len(tags))
	for _, tag := range tags {
		lookup[tag.Name] = tag.Value
	}
	game := NewGame()
	if fen := lookup["FEN"]; fen != "" {
		position, err := ParseFEN(fen)
		if err != nil {
			return nil, fmt.Errorf("invalid PGN FEN: %w", err)
		}
		game = NewGameFromPosition(position)
	} else if lookup["SetUp"] == "1" {
		return nil, fmt.Errorf("PGN SetUp requires a FEN tag")
	}
	game.tags = tags
	tokens, err := pgnTokens(movetext)
	if err != nil {
		return nil, err
	}
	result := ""
	for index, token := range tokens {
		if isResult(token) {
			if index != len(tokens)-1 {
				return nil, fmt.Errorf("PGN result must end movetext")
			}
			result = token
			break
		}
		if err := game.PlaySAN(token); err != nil {
			return nil, fmt.Errorf("PGN move %q: %w", token, err)
		}
	}
	if tagged := lookup["Result"]; tagged != "" && result != "" && tagged != result {
		return nil, fmt.Errorf("PGN result tag %q disagrees with movetext %q", tagged, result)
	}
	if result == "" {
		result = lookup["Result"]
	}
	if result != "" && !isResult(result) {
		return nil, fmt.Errorf("invalid PGN result %q", result)
	}
	if result != "" {
		actual := game.Result()
		if actual != "*" && actual != result {
			return nil, fmt.Errorf("PGN result %q disagrees with position result %q", result, actual)
		}
		if result != "*" {
			game.result = result
		}
	}
	return game, nil
}

func splitPGN(value string) ([]PGNTag, string, error) {
	var tags []PGNTag
	seen := make(map[string]bool)
	var movetext strings.Builder
	movesStarted := false
	for _, raw := range strings.Split(value, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") {
			if movesStarted {
				return nil, "", fmt.Errorf("PGN tag appears after movetext")
			}
			tag, err := parsePGNTag(line)
			if err != nil {
				return nil, "", err
			}
			if seen[tag.Name] {
				return nil, "", fmt.Errorf("duplicate PGN tag %q", tag.Name)
			}
			seen[tag.Name] = true
			tags = append(tags, tag)
			continue
		}
		if line != "" {
			movesStarted = true
		}
		movetext.WriteString(raw)
		movetext.WriteByte('\n')
	}
	return tags, movetext.String(), nil
}

func parsePGNTag(line string) (PGNTag, error) {
	space := strings.IndexAny(line, " \t")
	if space < 2 || !strings.HasSuffix(line, "]") {
		return PGNTag{}, fmt.Errorf("invalid PGN tag %q", line)
	}
	name := line[1:space]
	for _, char := range name {
		if !isPGNSymbolChar(char) {
			return PGNTag{}, fmt.Errorf("invalid PGN tag %q", line)
		}
	}
	encoded := strings.TrimSpace(line[space : len(line)-1])
	value, err := unescapePGNValue(encoded)
	if err != nil {
		return PGNTag{}, fmt.Errorf("invalid PGN tag %q", line)
	}
	return PGNTag{Name: name, Value: value}, nil
}

func isPGNSymbolChar(char rune) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("_+#=:-", char)
}

func unescapePGNValue(encoded string) (string, error) {
	if len(encoded) < 2 || encoded[0] != '"' || encoded[len(encoded)-1] != '"' {
		return "", fmt.Errorf("value is not quoted")
	}
	var value strings.Builder
	escaped := false
	for _, char := range encoded[1 : len(encoded)-1] {
		if escaped {
			if char != '\\' && char != '"' {
				return "", fmt.Errorf("invalid escape")
			}
			value.WriteRune(char)
			escaped = false
		} else if char == '\\' {
			escaped = true
		} else if char == '"' {
			return "", fmt.Errorf("unescaped quote")
		} else {
			value.WriteRune(char)
		}
	}
	if escaped {
		return "", fmt.Errorf("unfinished escape")
	}
	return value.String(), nil
}

func escapePGNValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func pgnTokens(value string) ([]string, error) {
	var clean strings.Builder
	braceDepth, variationDepth, lineComment := 0, 0, false
	for _, char := range value {
		switch {
		case lineComment && char != '\n':
			continue
		case char == '\n':
			lineComment = false
			if braceDepth == 0 && variationDepth == 0 {
				clean.WriteByte(' ')
			}
		case char == ';' && braceDepth == 0 && variationDepth == 0:
			lineComment = true
		case char == '{':
			if braceDepth != 0 {
				return nil, fmt.Errorf("nested PGN comment")
			}
			braceDepth++
		case char == '}':
			if braceDepth == 0 {
				return nil, fmt.Errorf("unmatched PGN comment terminator")
			}
			braceDepth--
		case braceDepth > 0:
			continue
		case char == '(':
			variationDepth++
		case char == ')':
			if variationDepth == 0 {
				return nil, fmt.Errorf("unmatched PGN variation terminator")
			}
			variationDepth--
		case variationDepth == 0:
			clean.WriteRune(char)
		}
	}
	if braceDepth != 0 {
		return nil, fmt.Errorf("unterminated PGN comment")
	}
	if variationDepth != 0 {
		return nil, fmt.Errorf("unterminated PGN variation")
	}
	var tokens []string
	for _, token := range strings.Fields(clean.String()) {
		if dollar := strings.IndexByte(token, '$'); dollar >= 0 {
			token = token[:dollar]
		}
		if dot := strings.LastIndexByte(token, '.'); dot >= 0 {
			token = token[dot+1:]
		}
		if token != "" && token != "e.p." && !strings.HasPrefix(token, "$") {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

func isResult(value string) bool {
	return value == "1-0" || value == "0-1" || value == "1/2-1/2" || value == "*"
}
