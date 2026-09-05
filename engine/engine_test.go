package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"chess-go"
)

var _ chess.Player = (*Bot)(nil)

func TestMaterialEvaluation(t *testing.T) {
	evaluator := MaterialEvaluator{}
	if got := evaluator.Evaluate(chess.NewPosition()); got != 0 {
		t.Fatalf("initial evaluation = %d", got)
	}
	position, _ := chess.ParseFEN("4k3/8/8/8/8/8/3Q4/4K3 w - - 0 1")
	if got := evaluator.Evaluate(position); got != 900 {
		t.Fatalf("queen advantage = %d", got)
	}
}

func TestPositionalEvaluationAddsStructureTerms(t *testing.T) {
	evaluator := PositionalEvaluator{}
	initial := evaluator.Evaluate(chess.NewPosition())
	central, _ := chess.ParseFEN("4k3/8/8/3pp3/3PP3/8/8/4K3 w - - 0 1")
	if got := evaluator.Evaluate(central); got == 0 || initial == got {
		t.Fatalf("positional evaluator did not add terms: initial %d central %d", initial, got)
	}
	passed, _ := chess.ParseFEN("4k3/8/8/4P3/8/8/8/4K3 w - - 0 1")
	blocked, _ := chess.ParseFEN("4k3/8/4p3/4P3/8/8/8/4K3 w - - 0 1")
	if evaluator.Evaluate(passed) <= evaluator.Evaluate(blocked) {
		t.Fatal("passed pawn was not rewarded")
	}
}

func TestEndgameEvaluationRewardsKingActivityAndPawnSupport(t *testing.T) {
	evaluator := EndgameEvaluator{}
	central, _ := chess.ParseFEN("4k3/8/8/3P4/3K4/8/8/8 w - - 0 1")
	corner, _ := chess.ParseFEN("4k3/8/8/3P4/8/8/8/7K w - - 0 1")
	if evaluator.Evaluate(central) <= evaluator.Evaluate(corner) {
		t.Fatalf("active king was not rewarded: central %d corner %d", evaluator.Evaluate(central), evaluator.Evaluate(corner))
	}
	if _, ok := NewProfile(Advanced).Evaluator.(EndgameEvaluator); !ok {
		t.Fatal("advanced profile did not use endgame evaluator")
	}
}

func TestBotChoosesMaterialAndLeavesInputUnchanged(t *testing.T) {
	position, _ := chess.ParseFEN("4k3/8/8/8/3q4/8/3R4/4K3 w - - 0 1")
	hash := position.Hash()
	move, err := New(1).ChooseMove(context.Background(), position)
	if err != nil {
		t.Fatal(err)
	}
	if got := move.UCI(); got != "d2d4" {
		t.Fatalf("ChooseMove() = %s, want d2d4", got)
	}
	if position.Hash() != hash {
		t.Fatal("search mutated its input")
	}
	if _, err := position.Apply(move); err != nil {
		t.Fatalf("bot returned illegal move: %v", err)
	}
}

func TestBotFindsMate(t *testing.T) {
	position, _ := chess.ParseFEN("7k/5Q2/6K1/8/8/8/8/8 w - - 0 1")
	move, err := New(2).ChooseMove(context.Background(), position)
	if err != nil {
		t.Fatal(err)
	}
	next, err := position.Apply(move)
	if err != nil || !next.InCheck() || len(next.LegalMoves()) != 0 {
		t.Fatalf("ChooseMove() = %s, not mate", move.UCI())
	}
}

func TestBotCancellationAndTerminalPosition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(3).ChooseMove(ctx, chess.NewPosition()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled search error = %v", err)
	}
	position, _ := chess.ParseFEN("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1")
	if _, err := New(1).ChooseMove(context.Background(), position); !errors.Is(err, ErrNoLegalMoves) {
		t.Fatalf("terminal search error = %v", err)
	}
}

func TestIterativeSearchAndNodeLimit(t *testing.T) {
	position := chess.NewPosition()
	move, stats, err := New(4).Search(context.Background(), position, SearchLimits{MaxDepth: 4})
	if err != nil || stats.Depth != 4 || stats.Nodes == 0 || stats.ReducedNodes == 0 || stats.NullCutoffs == 0 {
		t.Fatalf("iterative search = %s, %#v, %v", move.UCI(), stats, err)
	}
	if _, err := position.Apply(move); err != nil {
		t.Fatalf("iterative search returned illegal move: %v", err)
	}
	limitedMove, limited, err := New(4).Search(context.Background(), position, SearchLimits{MaxDepth: 4, MaxNodes: 45})
	if err != nil || limited.Depth != 1 || limited.Nodes > 45 {
		t.Fatalf("limited search = %s, %#v, %v", limitedMove.UCI(), limited, err)
	}
	if _, err := position.Apply(limitedMove); err != nil {
		t.Fatalf("limited search returned illegal move: %v", err)
	}
}

func TestSearchTimeAndContextLimits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := New(3).Search(ctx, chess.NewPosition(), SearchLimits{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled search error = %v", err)
	}
	if _, _, err := New(5).Search(context.Background(), chess.NewPosition(), SearchLimits{MaxDepth: 5, Time: time.Nanosecond}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed search error = %v", err)
	}
}

func TestTranspositionTableCachesCompletedSearch(t *testing.T) {
	position := chess.NewPosition()
	bot := New(3)
	control := &searchControl{table: make(map[uint64]ttEntry)}
	score, err := bot.search(context.Background(), MaterialEvaluator{}, &position, 3, 1, -infinity, infinity, control)
	if err != nil || len(control.table) == 0 {
		t.Fatalf("search table = %d entries, score %d, error %v", len(control.table), score, err)
	}
	firstNodes := control.nodes
	control.nodes = 0
	cached, err := bot.search(context.Background(), MaterialEvaluator{}, &position, 2, 1, -infinity, infinity, control)
	if err != nil || cached != score || control.nodes != 1 || firstNodes <= control.nodes {
		t.Fatalf("cached search = score %d nodes %d, first score %d nodes %d, error %v", cached, control.nodes, score, firstNodes, err)
	}
	if position.Hash() != chess.NewPosition().Hash() {
		t.Fatal("transposition search mutated its input")
	}
}

func TestTranspositionTablePrefersDeeperCollidingEntries(t *testing.T) {
	table := newTranspositionTable(1)
	table.store(1, ttEntry{depth: 4, score: 10})
	table.store(2, ttEntry{depth: 2, score: 20})
	if _, ok := table.lookup(2); ok {
		t.Fatal("shallower colliding entry replaced a deeper entry")
	}
	table.store(2, ttEntry{depth: 5, score: 30})
	entry, ok := table.lookup(2)
	if !ok || entry.depth != 5 || entry.score != 30 {
		t.Fatalf("deeper replacement = %#v, found %v", entry, ok)
	}
}

func TestQuiescenceDeltaPruningCountsSafeCaptureRejections(t *testing.T) {
	position, err := chess.ParseFEN("4k3/8/8/8/8/3p4/2P5/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	control := &searchControl{}
	if _, err := New(1).quiescence(context.Background(), MaterialEvaluator{}, &position, 0, 500, infinity, control); err != nil {
		t.Fatal(err)
	}
	if control.deltaPrunes == 0 {
		t.Fatal("quiescence did not record a safe delta prune")
	}
}

func TestStrengthProfiles(t *testing.T) {
	want := []struct {
		name  string
		level StrengthProfile
		depth int
		loss  Score
	}{
		{"Learner", Learner, 1, 300}, {"Beginner", Beginner, 2, 200},
		{"Casual", Casual, 2, 100}, {"Club", Club, 3, 50},
		{"Advanced", Advanced, 3, 25}, {"Expert", Expert, 4, 10}, {"Maximum", Maximum, 4, 0},
	}
	for _, test := range want {
		profile, err := ParseStrengthProfile(strings.ToLower(test.name))
		if err != nil || profile != test.level || profile.String() != test.name {
			t.Fatalf("profile %q = %v, %v", test.name, profile, err)
		}
		bot := NewProfile(profile)
		if bot.Depth != test.depth || bot.MaxLoss != test.loss || bot.Strength != test.level {
			t.Fatalf("config %q = depth %d loss %d level %v", test.name, bot.Depth, bot.MaxLoss, bot.Strength)
		}
		if profile < Maximum && (bot.InaccuracyChance <= 0 || bot.TacticalAwareness <= 0) {
			t.Fatalf("profile %q has no calibrated imperfection settings", test.name)
		}
	}
	if _, err := ParseStrengthProfile("unrated"); err == nil {
		t.Fatal("unknown profile accepted")
	}
	if got := Profiles(); len(got) != 7 || got[0] != Learner || got[6] != Maximum {
		t.Fatalf("profiles = %#v", got)
	}
}

func TestRandomBotSamplesNearBestMoves(t *testing.T) {
	position := chess.NewPosition()
	seen := make(map[string]struct{})
	for seed := uint64(1); seed <= 8; seed++ {
		bot := NewRandom(1, seed)
		move, err := bot.ChooseMove(context.Background(), position)
		if err != nil {
			t.Fatal(err)
		}
		seen[move.UCI()] = struct{}{}
	}
	if len(seen) < 2 {
		t.Fatalf("random bot selected only one opening move: %v", seen)
	}
	first, err := NewRandom(1, 42).ChooseMove(context.Background(), position)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRandom(1, 42).ChooseMove(context.Background(), position)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("same seed selected %s then %s", first.UCI(), second.UCI())
	}
}

func TestPersonalityStyleBonusesArePositionAware(t *testing.T) {
	position, err := chess.ParseFEN("4k3/8/8/8/3q4/8/3R4/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	var capture, quiet chess.Move
	for _, move := range position.LegalMoves() {
		if move.UCI() == "d2d4" {
			capture = move
		}
		if move.UCI() == "d2d3" {
			quiet = move
		}
	}
	if styleBonus(position, capture, Aggressive) <= styleBonus(position, quiet, Aggressive) {
		t.Fatal("aggressive style did not prefer a forcing capture")
	}
	if !tacticalPosition(position) {
		t.Fatal("capture position was not classified as tactical")
	}
	quietPosition, err := chess.ParseFEN("4k3/8/8/8/8/8/3P4/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if tacticalPosition(quietPosition) {
		t.Fatal("quiet position was classified as tactical")
	}
}

func TestPersonalitiesAndSeededCandidateSelection(t *testing.T) {
	for _, name := range []string{"cautious", "aggressive", "materialist", "tactician", "positional", "simplifier", "trickster"} {
		personality, err := ParsePersonality(name)
		if err != nil || strings.ToLower(personality.String()) != name {
			t.Fatalf("personality %q = %v, %v", name, personality, err)
		}
	}
	candidates := []scoredMove{
		{move: chess.Move{From: 1, To: 2}, score: 100},
		{move: chess.Move{From: 2, To: 3}, score: 95},
		{move: chess.Move{From: 3, To: 4}, score: 90},
	}
	first := chooseCandidate(candidates, 100, 20, &searchControl{random: 7})
	second := chooseCandidate(candidates, 100, 20, &searchControl{random: 7})
	if first != second {
		t.Fatalf("same seed selected %s then %s", first.UCI(), second.UCI())
	}
	bot := New(1)
	bot.SetPersonality(Trickster, 0)
	if bot.Seed == 0 || bot.Temperature != Trickster.Config().Temperature {
		t.Fatalf("personality config = seed %x temperature %v", bot.Seed, bot.Temperature)
	}
	if _, err := ParsePersonality("random"); err == nil {
		t.Fatal("unknown personality accepted")
	}
}

func TestOpeningBookUsesLegalEntriesAndFallsBack(t *testing.T) {
	position := chess.NewPosition()
	profile := NewProfile(Learner)
	move, err := profile.ChooseMove(context.Background(), position)
	if err != nil || move.UCI() != "e2e4" {
		t.Fatalf("built-in opening move = %s, %v", move.UCI(), err)
	}
	invalid := HashBook{position.Hash(): chess.Move{From: chess.Square(0), To: chess.Square(0)}}
	bot := New(1)
	bot.Book = invalid
	move, err = bot.ChooseMove(context.Background(), position)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := position.Apply(move); err != nil {
		t.Fatalf("book fallback move = %s: %v", move.UCI(), err)
	}
}
