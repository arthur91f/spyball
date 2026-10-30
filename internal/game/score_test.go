package game

import (
	"slices"
	"testing"

	"github.com/arthur91f/spyball/internal/config"
)

// handGame builds a running game whose players have the given colors, rules and choices,
// named after their IDs for readable tests.
func handGame(players ...*Player) *Game {
	for _, p := range players {
		p.Name = string(p.ID)
	}
	return &Game{Phase: PhaseRunning, Players: players}
}

func TestRelation(t *testing.T) {
	blue := &Player{ID: "blue", Color: config.Blue}
	blue2 := &Player{ID: "blue2", Color: config.Blue}
	blueTraitor := &Player{ID: "blueTraitor", Color: config.Blue, Traitor: true}
	red := &Player{ID: "red", Color: config.Red}
	redTraitor := &Player{ID: "redTraitor", Color: config.Red, Traitor: true}

	tests := []struct {
		a, b *Player
		want Guess
	}{
		{blue, blue2, GuessAlly},
		{blue, red, GuessEnemy},
		{blue, blueTraitor, GuessAlly}, // a traitor is an ordinary member of its color
		{red, blueTraitor, GuessEnemy},
		{blueTraitor, redTraitor, GuessEnemy},
		{red, redTraitor, GuessAlly},
	}
	for _, tt := range tests {
		if got := Relation(tt.a, tt.b); got != tt.want {
			t.Errorf("Relation(%s, %s) = %s, want %s", tt.a.ID, tt.b.ID, got, tt.want)
		}
		if got := Relation(tt.b, tt.a); got != tt.want {
			t.Errorf("Relation(%s, %s) = %s, want %s (symmetry)", tt.b.ID, tt.a.ID, got, tt.want)
		}
	}
}

func TestClues(t *testing.T) {
	g := handGame(
		&Player{ID: "b", Color: config.Blue, Character: "USA", Sign: "bs", Clues: []PlayerID{"t", "r"}},
		&Player{ID: "t", Color: config.Blue, Character: "France", Sign: "ts", Traitor: true},
		&Player{ID: "r", Color: config.Red, Character: "Russie", Sign: "rs", Clues: []PlayerID{"b"}},
	)

	tests := []struct {
		viewer PlayerID
		want   []Clue
	}{
		{"b", []Clue{{Color: config.Blue, Sign: "ts"}, {Color: config.Red, Sign: "rs"}}}, // nothing tells t is a traitor
		{"t", []Clue{}},
		{"r", []Clue{{Color: config.Blue, Sign: "bs"}}},
	}
	for _, tt := range tests {
		got, err := g.Clues(tt.viewer)
		if err != nil {
			t.Fatalf("Clues(%s): %v", tt.viewer, err)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("Clues(%s) = %v, want %v", tt.viewer, got, tt.want)
		}
	}
}

func TestScore(t *testing.T) {
	g := handGame(
		&Player{ID: "b1", Color: config.Blue, Traitor: true},
		&Player{ID: "b2", Color: config.Blue},
		&Player{ID: "b3", Color: config.Blue},
		&Player{ID: "r1", Color: config.Red},
		&Player{ID: "r2", Color: config.Red, Traitor: true},
		&Player{ID: "r3", Color: config.Red},
	)
	guesses := []struct {
		from, about PlayerID
		guess       Guess
	}{
		{"b1", "b2", GuessAlly},  // ally found, b2 designated as ally
		{"b1", "r1", GuessEnemy}, // enemy found, r1 designated as enemy
		{"b1", "b3", GuessEnemy}, // wrong, b3 designated as enemy all the same
		{"b2", "b1", GuessAlly},  // ally found, b1 designated as ally
		{"b2", "r2", GuessAlly},  // wrong, r2 designated as ally all the same
		{"r1", "b1", GuessEnemy}, // enemy found, b1 designated as enemy
		{"r1", "r3", GuessAlly},  // ally found, r3 designated as ally
		{"r3", "r1", GuessAlly},  // ally found, r1 designated as ally
		{"r2", "b2", GuessEnemy}, // enemy found, b2 designated as enemy
	}
	for _, gs := range guesses {
		if err := g.SetGuess(gs.from, gs.about, gs.guess); err != nil {
			t.Fatalf("SetGuess(%s, %s): %v", gs.from, gs.about, err)
		}
	}
	g.Challenges = []Challenge{{Text: "c1"}, {Text: "c2"}, {Text: "c3"}}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	_ = g.AddManualPoints("b3", -1, "dénoncé")
	_ = g.SetChallengesDone([]bool{true, false, false}) // scores for the traitors' camp

	res := g.Score()

	wantPlayers := map[PlayerID]PlayerScore{
		// 2 + 2 + 0 (wrong) + 1 (designated ally) − 2 (designated enemy); challenges score for the traitors' camp
		"b1": {Player: "b1", Color: config.Blue, Traitor: true, AlliesFound: 1, EnemiesFound: 1, Wrong: 1, DesignatedAlly: 1, DesignatedEnemy: 1, Total: 3},
		"b2": {Player: "b2", Color: config.Blue, AlliesFound: 1, Wrong: 1, DesignatedAlly: 1, DesignatedEnemy: 1, Total: 1},
		"b3": {Player: "b3", Color: config.Blue, DesignatedEnemy: 1, Manual: -1, Total: -3}, // scores can be negative
		"r1": {Player: "r1", Color: config.Red, AlliesFound: 1, EnemiesFound: 1, DesignatedAlly: 1, DesignatedEnemy: 1, Total: 3},
		"r2": {Player: "r2", Color: config.Red, Traitor: true, EnemiesFound: 1, DesignatedAlly: 1, Total: 3},
		"r3": {Player: "r3", Color: config.Red, AlliesFound: 1, DesignatedAlly: 1, Total: 3},
	}
	if len(res.Players) != len(wantPlayers) {
		t.Fatalf("%d player scores, want %d", len(res.Players), len(wantPlayers))
	}
	for i, got := range res.Players {
		if got.Player != g.Players[i].ID {
			t.Errorf("player scores not in game order at %d", i)
		}
		if want := wantPlayers[got.Player]; got != want {
			t.Errorf("score of %s = %+v, want %+v", got.Player, got, want)
		}
	}

	wantTeams := []TeamScore{
		{Color: config.Blue, Members: 2, Score: -2}, // traitors do not count for their color
		{Color: config.Red, Members: 2, Score: 6},
		{Traitors: true, Members: 2, Challenges: 1, Score: 3 + 3 + PointsChallenge}, // traitors and challenges done
	}
	if !slices.Equal(res.Teams, wantTeams) {
		t.Errorf("teams = %+v, want %+v", res.Teams, wantTeams)
	}
}
