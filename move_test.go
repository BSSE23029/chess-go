package chess

import (
	"strings"
	"testing"
)

func TestApplyMoveLifecycle(t *testing.T) {
	position := NewPosition()
	for _, uci := range []string{"e2e4", "c7c5", "e4e5", "d7d5", "e5d6"} {
		var err error
		position, err = position.ApplyUCI(uci)
		if err != nil {
			t.Fatalf("ApplyUCI(%q): %v", uci, err)
		}
	}
	if got := position.FEN(); got != "rnbqkbnr/pp2pppp/3P4/2p5/8/8/PPPP1PPP/RNBQKBNR b KQkq - 0 3" {
		t.Fatalf("FEN() = %q", got)
	}
	if _, err := position.ApplyUCI("e8e7"); err == nil {
		t.Fatal("illegal move accepted")
	}
}

func TestEnPassantRequiresCapturablePawn(t *testing.T) {
	if _, err := ParseFEN("7k/8/8/4P3/8/8/8/K7 w - d6 0 1"); err == nil {
		t.Fatal("accepted en passant target without a capturable pawn")
	}
}

func TestCastlingAndPromotion(t *testing.T) {
	position, _ := ParseFEN("r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")
	game := NewGameFromPosition(position)
	if err := game.PlayUCI("e1g1"); err != nil || game.Moves()[0].Flags&Castle == 0 {
		t.Fatalf("game did not retain castling move: %v", err)
	}
	next, err := position.ApplyUCI("e1g1")
	if err != nil {
		t.Fatal(err)
	}
	if got := next.FEN(); got != "r3k2r/8/8/8/8/8/8/R4RK1 b kq - 1 1" {
		t.Fatalf("castling FEN = %q", got)
	}

	position, _ = ParseFEN("7k/P7/8/8/8/8/8/7K w - - 0 1")
	next, err = position.ApplyUCI("a7a8q")
	if err != nil {
		t.Fatal(err)
	}
	if got := next.FEN(); got != "Q6k/8/8/8/8/8/8/7K b - - 0 1" {
		t.Fatalf("promotion FEN = %q", got)
	}
}

func TestApplyDoesNotMutatePosition(t *testing.T) {
	position := NewPosition()
	if _, err := position.ApplyUCI("e2e4"); err != nil {
		t.Fatal(err)
	}
	if got := position.FEN(); got != InitialFEN {
		t.Fatalf("original mutated: %q", got)
	}
}

func TestLegalMovesIntoMatchesPublicMoveGeneration(t *testing.T) {
	positions := []Position{NewPosition()}
	for _, fen := range []string{
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
		"7k/P7/8/8/8/8/8/7K w - - 0 1",
	} {
		position, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		positions = append(positions, position)
	}
	for _, position := range positions {
		want := position.LegalMoves()
		buffer := make([]Move, 0, 256)
		got := position.LegalMovesInto(buffer)
		if len(got) != len(want) {
			t.Fatalf("LegalMovesInto() returned %d moves, want %d", len(got), len(want))
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("LegalMovesInto()[%d] = %s, want %s", index, got[index].UCI(), want[index].UCI())
			}
		}
	}
}

func TestMakeUnmakeMoveLifecycle(t *testing.T) {
	tests := []struct {
		name, fen, move string
	}{
		{"pawn double", InitialFEN, "e2e4"},
		{"capture", "4k3/8/8/8/3q4/8/3R4/4K3 w - - 0 1", "d2d4"},
		{"en passant", "7k/8/8/3pP3/8/8/8/K7 w - d6 0 2", "e5d6"},
		{"castling", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1"},
		{"promotion", "7k/P7/8/8/8/8/8/7K w - - 0 1", "a7a8q"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			before, hash := position.FEN(), position.Hash()
			move, _ := ParseUCI(test.move)
			undo, err := position.MakeMove(move)
			if err != nil || position.FEN() == before {
				t.Fatalf("MakeMove(%s) failed: %v", test.move, err)
			}
			if position.Hash() != position.calculateHash() {
				t.Fatalf("MakeMove(%s) produced an incorrect incremental hash", test.move)
			}
			position.UnmakeMove(undo)
			if position.FEN() != before || position.Hash() != hash {
				t.Fatalf("UnmakeMove(%s) restored %q, want %q", test.move, position.FEN(), before)
			}
		})
	}

	position := NewPosition()
	before := position.FEN()
	move, _ := ParseUCI("e2e5")
	if _, err := position.MakeMove(move); err == nil || position.FEN() != before {
		t.Fatal("invalid MakeMove changed the position")
	}
	position.UnmakeMove(Undo{})
	if position.FEN() != before {
		t.Fatal("zero Undo changed the position")
	}
}

func TestNestedMakeUnmake(t *testing.T) {
	position := NewPosition()
	initial := position.FEN()
	e4, _ := ParseUCI("e2e4")
	first, _ := position.MakeMove(e4)
	afterE4 := position.FEN()
	e5, _ := ParseUCI("e7e5")
	second, _ := position.MakeMove(e5)
	position.UnmakeMove(second)
	if position.FEN() != afterE4 {
		t.Fatal("inner unmake did not restore parent position")
	}
	position.UnmakeMove(first)
	if position.FEN() != initial {
		t.Fatal("outer unmake did not restore initial position")
	}
}

func TestInitialPosition(t *testing.T) {
	position := NewPosition()
	if got := position.FEN(); got != InitialFEN {
		t.Fatalf("FEN() = %q, want %q", got, InitialFEN)
	}
	e1, _ := ParseSquare("e1")
	if got := position.PieceAt(e1); got != (Piece{Type: King, Color: White}) {
		t.Fatalf("PieceAt(e1) = %+v", got)
	}
	if position.Turn() != White || position.Castling() != WhiteKingSide|WhiteQueenSide|BlackKingSide|BlackQueenSide {
		t.Fatal("initial metadata is incorrect")
	}
}

func TestFENRoundTrip(t *testing.T) {
	const fen = "r3k2r/ppp2ppp/2n1bn2/3qp3/3P4/2N1PN2/PPP2PPP/R2QKB1R b KQkq d3 0 12"
	position, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	if got := position.FEN(); got != fen {
		t.Fatalf("FEN() = %q, want %q", got, fen)
	}
	if position.Turn() != Black || position.EnPassant().String() != "d3" || position.HalfmoveClock() != 0 || position.FullmoveNumber() != 12 {
		t.Fatal("parsed metadata is incorrect")
	}
}

func TestParseFENRejectsInvalidInput(t *testing.T) {
	tests := []string{
		"8/8/8/8/8/8/8/8 w - - 0",
		"8/8/8/8/8/8/8 w - - 0 1",
		"8/8/8/8/8/8/8/9 w - - 0 1",
		"8/8/8/8/8/8/8/8 x - - 0 1",
		"8/8/8/8/8/8/8/8 w KK - 0 1",
		"8/8/8/8/8/8/8/8 w - e4 0 1",
		"8/8/8/8/8/8/8/8 w - - -1 1",
		"8/8/8/8/8/8/8/8 w - - 0 0",
	}
	for _, fen := range tests {
		if _, err := ParseFEN(fen); err == nil {
			t.Errorf("ParseFEN(%q) succeeded", fen)
		}
	}
}

func TestParseFENSemanticValidation(t *testing.T) {
	invalid := []string{
		"8/8/8/8/8/8/8/K7 w - - 0 1",
		"7k/8/8/8/8/8/8/KK6 w - - 0 1",
		"7k/8/8/8/8/8/8/6K1 w K - 0 1",
		"7k/8/8/8/8/8/PPPPPPPP/P3K3 w - - 0 1",
		"7k/8/8/8/8/QQ6/PPPPPPPP/4K3 w - - 0 1",
		"4k3/8/8/8/8/8/4R3/4K3 w - - 0 1",
		"7k/8/8/3pP3/8/8/8/K7 w - d3 0 1",
		"7k/8/8/3pP3/8/8/8/K7 w - d6 1 1",
	}
	for _, fen := range invalid {
		if _, err := ParseFEN(fen); err == nil {
			t.Errorf("ParseFEN(%q) accepted an impossible position", fen)
		}
	}
	valid := []string{
		InitialFEN,
		"k3r3/8/8/8/8/8/8/4K3 w - - 0 1",
		"7k/8/8/3p4/8/8/8/K7 w - d6 0 2",
	}
	for _, fen := range valid {
		if _, err := ParseFEN(fen); err != nil {
			t.Errorf("ParseFEN(%q) rejected a valid position: %v", fen, err)
		}
	}
}

func TestSquareRoundTrip(t *testing.T) {
	for _, value := range []string{"a1", "e4", "h8"} {
		square, err := ParseSquare(value)
		if err != nil || square.String() != value {
			t.Fatalf("ParseSquare(%q) = %v, %v", value, square, err)
		}
	}
	for _, value := range []string{"", "a0", "i1", "a10"} {
		if _, err := ParseSquare(value); err == nil {
			t.Errorf("ParseSquare(%q) succeeded", value)
		}
	}
}

func TestGamePlayUndoRedoLifecycle(t *testing.T) {
	game := NewGame()
	for _, move := range []string{"e2e4", "e7e5", "g1f3"} {
		if err := game.PlayUCI(move); err != nil {
			t.Fatalf("PlayUCI(%q): %v", move, err)
		}
	}
	played := game.Moves()
	played[0] = Move{}
	if game.Moves()[0].UCI() != "e2e4" {
		t.Fatal("Moves exposed mutable game history")
	}
	after := game.Position().FEN()
	if !game.Undo() || game.Position().Turn() != White || !game.CanRedo() {
		t.Fatal("undo did not restore the prior turn")
	}
	if !game.Redo() || game.Position().FEN() != after {
		t.Fatal("redo did not restore the position")
	}
	game.Undo()
	if err := game.PlayUCI("d2d4"); err != nil {
		t.Fatal(err)
	}
	if game.CanRedo() || len(game.Moves()) != 3 {
		t.Fatal("new play did not replace the redo branch")
	}
	if err := game.PlayUCI("e1e8"); err == nil {
		t.Fatal("illegal game move accepted")
	}
}

func TestCapturedPiecesFollowHistoryCursor(t *testing.T) {
	position, _ := ParseFEN("7k/8/8/3pP3/8/8/8/K7 w - d6 0 2")
	game := NewGameFromPosition(position)
	if err := game.PlayUCI("e5d6"); err != nil {
		t.Fatal(err)
	}
	captured := game.Captured()
	if len(captured) != 1 || captured[0] != (Piece{Type: Pawn, Color: Black}) {
		t.Fatalf("en-passant captures = %#v", captured)
	}
	game.Undo()
	if len(game.Captured()) != 0 {
		t.Fatal("undone capture remained visible")
	}
	game.Redo()
	if len(game.Captured()) != 1 {
		t.Fatal("redone capture was not restored")
	}
}

func TestGameCheckmateAndStalemate(t *testing.T) {
	game := NewGame()
	for _, move := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		if err := game.PlayUCI(move); err != nil {
			t.Fatal(err)
		}
	}
	if got := game.Status(); got != BlackCheckmates {
		t.Fatalf("Status() = %v, want BlackCheckmates", got)
	}
	if err := game.PlayUCI("a2a3"); err == nil {
		t.Fatal("move accepted after checkmate")
	}

	position, _ := ParseFEN("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1")
	if got := NewGameFromPosition(position).Status(); got != Stalemate {
		t.Fatalf("Status() = %v, want Stalemate", got)
	}
}

func TestGameDrawRules(t *testing.T) {
	position, _ := ParseFEN("7k/8/8/8/8/8/8/R6K w - - 99 1")
	game := NewGameFromPosition(position)
	if err := game.PlayUCI("a1a2"); err != nil {
		t.Fatal(err)
	}
	if got := game.Status(); got != DrawFiftyMove {
		t.Fatalf("Status() = %v, want DrawFiftyMove", got)
	}

	game = NewGame()
	for _, move := range []string{"g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"} {
		if err := game.PlayUCI(move); err != nil {
			t.Fatal(err)
		}
	}
	if got := game.Status(); got != DrawThreefoldRepetition {
		t.Fatalf("Status() = %v, want DrawThreefoldRepetition", got)
	}
}

func TestFIDERulesSeparateClaimsFromAutomaticDraws(t *testing.T) {
	claimPosition, err := ParseFEN("7k/8/8/8/8/8/8/R5K1 w - - 100 1")
	if err != nil {
		t.Fatal(err)
	}
	claimed := NewGameFromPosition(claimPosition)
	if claimed.ClaimableDraw() != DrawFiftyMove || !claimed.CanClaimDraw() {
		t.Fatalf("claimable status = %v", claimed.ClaimableDraw())
	}
	if err := claimed.ClaimDraw(); err != nil || claimed.ResultFIDE() != "1/2-1/2" {
		t.Fatalf("claim draw = %v, %q", err, claimed.ResultFIDE())
	}

	automaticPosition, err := ParseFEN("7k/8/8/8/8/8/8/R5K1 w - - 149 1")
	if err != nil {
		t.Fatal(err)
	}
	automatic := NewGameFromPosition(automaticPosition)
	if err := automatic.PlayFIDE(Move{From: 0, To: 8}); err != nil {
		t.Fatal(err)
	}
	if got := automatic.StatusWithRules(FIDERules); got != DrawSeventyFiveMove || automatic.ResultFIDE() != "1/2-1/2" {
		t.Fatalf("75-move status = %v, result %q", got, automatic.ResultFIDE())
	}

	repeated := NewGame()
	cycle := []string{"g1f3", "g8f6", "f3g1", "f6g8"}
	for index := 0; index < 4; index++ {
		for _, move := range cycle {
			if err := repeated.PlayUCIFIDE(move); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := repeated.StatusWithRules(FIDERules); got != DrawFivefoldRepetition {
		t.Fatalf("fivefold status = %v", got)
	}

	checkmate, err := ParseFEN("7k/6Q1/6K1/8/8/8/8/8 b - - 150 1")
	if err != nil {
		t.Fatal(err)
	}
	if got := NewGameFromPosition(checkmate).StatusWithRules(FIDERules); got != WhiteCheckmates {
		t.Fatalf("checkmate did not take precedence over 75-move draw: %v", got)
	}
}

func TestInsufficientMaterial(t *testing.T) {
	tests := []struct {
		fen  string
		draw bool
	}{
		{"7k/8/8/8/8/8/8/K7 w - - 0 1", true},
		{"7k/8/8/8/8/8/6N1/K7 w - - 0 1", true},
		{"5b1k/8/8/8/8/8/6B1/K7 w - - 0 1", false},
		{"6bk/8/8/8/8/8/6B1/K7 w - - 0 1", true},
		{"7k/8/8/8/8/8/6B1/KN6 w - - 0 1", false},
		{"7k/8/8/8/8/8/6P1/K7 w - - 0 1", false},
	}
	for _, test := range tests {
		position, err := ParseFEN(test.fen)
		if err != nil {
			t.Fatal(err)
		}
		got := NewGameFromPosition(position).Status() == DrawInsufficientMaterial
		if got != test.draw {
			t.Errorf("Status(%q) insufficient = %t, want %t", test.fen, got, test.draw)
		}
	}
}

func TestSANLifecycle(t *testing.T) {
	game, err := FromSAN([]string{"e4", "d5", "exd5", "Nf6"})
	if err != nil {
		t.Fatal(err)
	}
	if got := game.Position().FEN(); got != "rnbqkb1r/ppp1pppp/5n2/3P4/8/8/PPPP1PPP/RNBQKBNR w KQkq - 1 3" {
		t.Fatalf("SAN sequence FEN = %q", got)
	}
	if err := game.PlaySAN("not-a-move"); err == nil {
		t.Fatal("invalid SAN accepted")
	}

	position, _ := ParseFEN("4k3/8/8/8/8/5N2/8/1N2K3 w - - 0 1")
	move, _ := ParseUCI("b1d2")
	if got, err := position.SAN(move); err != nil || got != "Nbd2" {
		t.Fatalf("SAN(b1d2) = %q, %v", got, err)
	}
	parsed, err := position.ParseSAN("Nbd2")
	if err != nil || parsed.UCI() != "b1d2" {
		t.Fatalf("ParseSAN(Nbd2) = %s, %v", parsed.UCI(), err)
	}

	position, _ = ParseFEN("4k3/8/8/8/8/R7/8/R3K3 w - - 0 1")
	move, _ = ParseUCI("a1a2")
	if got, _ := position.SAN(move); got != "R1a2" {
		t.Fatalf("SAN(a1a2) = %q", got)
	}
}

func TestSANSpecialMovesAndMate(t *testing.T) {
	tests := []struct {
		fen, uci, san string
	}{
		{"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", "O-O"},
		{"7k/P7/8/8/8/8/8/7K w - - 0 1", "a7a8q", "a8=Q+"},
	}
	for _, test := range tests {
		position, _ := ParseFEN(test.fen)
		move, _ := ParseUCI(test.uci)
		if got, err := position.SAN(move); err != nil || got != test.san {
			t.Errorf("SAN(%s) = %q, %v; want %q", test.uci, got, err, test.san)
		}
		if parsed, err := position.ParseSAN(test.san); err != nil || parsed.UCI() != test.uci {
			t.Errorf("ParseSAN(%q) = %s, %v", test.san, parsed.UCI(), err)
		}
	}

	game, _ := FromSAN([]string{"f3", "e5", "g4"})
	move, _ := ParseUCI("d8h4")
	if got, err := game.Position().SAN(move); err != nil || got != "Qh4#" {
		t.Fatalf("mate SAN = %q, %v", got, err)
	}
}

func TestPGNRoundTrip(t *testing.T) {
	input := `[Event "Test"]
[Annotator "Raza \\ Team \"A\""]
[X-Custom "retained"]
[Result "0-1"]

1. f3 {weak move} e5
2. g4 (2. e4) Qh4# $1 0-1`
	game, err := ParsePGN(input)
	if err != nil {
		t.Fatal(err)
	}
	if game.Status() != BlackCheckmates || game.Result() != "0-1" || len(game.Moves()) != 4 {
		t.Fatal("parsed PGN lifecycle is incorrect")
	}
	tags := game.Tags()
	if len(tags) != 4 || tags[1].Name != "Annotator" || tags[1].Value != `Raza \ Team "A"` || tags[2].Value != "retained" {
		t.Fatalf("PGN tags not retained: %#v", tags)
	}
	tags[1].Value = "changed"
	if game.Tags()[1].Value == "changed" {
		t.Fatal("Tags exposed mutable game metadata")
	}
	exported := game.PGN()
	if !strings.Contains(exported, `[Annotator "Raza \\ Team \"A\""]`) || !strings.Contains(exported, `[X-Custom "retained"]`) {
		t.Fatalf("escaped or custom tags not exported:\n%s", exported)
	}
	reparsed, err := ParsePGN(exported)
	if err != nil {
		t.Fatal(err)
	}
	if reparsed.Position().FEN() != game.Position().FEN() || reparsed.Result() != game.Result() {
		t.Fatal("PGN round-trip changed the game")
	}
}

func TestPGNTagAndResultMutation(t *testing.T) {
	game := NewGame()
	if err := game.SetTag("Engine", "profile-a"); err != nil {
		t.Fatal(err)
	}
	if err := game.SetTag("Engine", "profile-b"); err != nil || len(game.Tags()) != 1 || game.Tags()[0].Value != "profile-b" {
		t.Fatalf("updated tag = %#v, %v", game.Tags(), err)
	}
	if err := game.SetTag("bad tag", "value"); err == nil {
		t.Fatal("invalid tag accepted")
	}
	if err := game.SetResult("1/2-1/2"); err != nil || game.Result() != "1/2-1/2" {
		t.Fatalf("declared result = %q, %v", game.Result(), err)
	}
	if err := game.SetResult("*"); err != nil || game.Result() != "*" {
		t.Fatalf("cleared result = %q, %v", game.Result(), err)
	}
}

func TestPGNCustomPositionAndValidation(t *testing.T) {
	const input = `[SetUp "1"]
[FEN "7k/7p/8/8/8/8/P7/K7 b - - 0 1"]
[Result "*"]

1... Kg8 *`
	game, err := ParsePGN(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := game.Position().FEN(); got != "6k1/7p/8/8/8/8/P7/K7 w - - 1 2" {
		t.Fatalf("custom PGN FEN = %q", got)
	}
	if output := game.PGN(); !strings.Contains(output, `[SetUp "1"]`) || !strings.Contains(output, "1... Kg8 *") {
		t.Fatalf("custom PGN not preserved:\n%s", output)
	}
	for _, invalid := range []string{
		`[SetUp "1"]` + "\n\n*",
		`[Result "1-0"]` + "\n\n1. f3 e5 2. g4 Qh4# 0-1",
		`[Result "bad"]` + "\n\n",
		`1. e4 1-0 e5`,
		`1. e4 {unfinished *`,
		`1. e4 {outer {nested} comment} *`,
		`1. e4 (1... e5 *`,
		`1. e4 } *`,
		`1. e4 ) *`,
		"[Event \"bad\\nvalue\"]\n\n*",
		"[Event \"one\"]\n[Event \"two\"]\n\n*",
		"1. e4\n[Event \"late\"]\n*",
	} {
		if _, err := ParsePGN(invalid); err == nil {
			t.Errorf("ParsePGN(%q) succeeded", invalid)
		}
	}
	resigned, err := ParsePGN("[Result \"1-0\"]\n\n1. e4 1-0")
	if err != nil || resigned.Result() != "1-0" {
		t.Fatalf("resignation result was not retained: %v", err)
	}
	if err := resigned.PlaySAN("e5"); err == nil {
		t.Fatal("move accepted after declared PGN result")
	}
}

func TestPositionHash(t *testing.T) {
	initial := NewPosition()
	if initial.Hash() != 0x63b5cf5c00a21380 {
		t.Fatalf("initial hash changed: %x", initial.Hash())
	}
	sameBoard, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 42 99")
	blackTurn, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 0 1")
	noCastling, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1")
	if initial.Hash() != sameBoard.Hash() {
		t.Fatal("move clocks changed position hash")
	}
	if initial.Hash() == blackTurn.Hash() || initial.Hash() == noCastling.Hash() {
		t.Fatal("side or castling rights did not change position hash")
	}
	if initial.Hash() != NewPosition().Hash() {
		t.Fatal("position hash is not deterministic")
	}
	for _, move := range initial.LegalMoves() {
		position := initial
		undo := position.MakeLegalMove(move)
		if position.Hash() != position.calculateHash() {
			t.Fatalf("incremental hash is wrong after %s", move.UCI())
		}
		position.UnmakeMove(undo)
		if position.Hash() != initial.Hash() {
			t.Fatalf("hash was not restored after %s", move.UCI())
		}
	}
}

func TestNullMoveIsSearchOnlyAndRehashes(t *testing.T) {
	initial := NewPosition()
	passed, err := initial.NullMove()
	if err != nil || passed.Turn() != Black || passed.EnPassant() != NoSquare || passed.HalfmoveClock() != 1 || passed.FullmoveNumber() != 1 {
		t.Fatalf("null move = %#v, %v", passed, err)
	}
	if passed.Hash() != passed.calculateHash() || initial.Turn() != White {
		t.Fatal("null move did not preserve hash/input invariants")
	}
	inCheck, _ := ParseFEN("4k3/8/8/8/8/8/4r3/4K3 w - - 0 1")
	if _, err := inCheck.NullMove(); err == nil {
		t.Fatal("null move allowed while in check")
	}
}
