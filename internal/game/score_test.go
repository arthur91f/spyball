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
	red := &Player{ID: "red", Color: config.Red}
	whiteNeutral := &Player{ID: "whiteNeutral", Color: config.White}
	whiteBlue := &Player{ID: "whiteBlue", Color: config.White, Choice: config.CampBlue}
	whiteRed := &Player{ID: "whiteRed", Color: config.White, Choice: config.CampRed}
	ong := &Player{ID: "ong", Color: config.Yellow, Rule: config.AllyOfEveryone}
	anon := &Player{ID: "anon", Color: config.Yellow, Rule: config.EnemyOfEveryone}

	tests := []struct {
		a, b *Player
		want Guess
	}{
		{blue, blue2, GuessAlly},
		{blue, red, GuessEnemy},
		{blue, whiteNeutral, GuessEnemy}, // a neutral is an enemy of blue and red
		{red, whiteNeutral, GuessEnemy},
		{blue, whiteBlue, GuessAlly}, // a white fully joins the chosen camp
		{red, whiteBlue, GuessEnemy},
		{whiteRed, whiteBlue, GuessEnemy},
		{whiteNeutral, whiteNeutral, GuessAlly},
		{blue, ong, GuessAlly}, // ally of everyone
		{red, ong, GuessAlly},
		{whiteNeutral, ong, GuessAlly},
		{blue, anon, GuessEnemy}, // enemy of everyone
		{whiteNeutral, anon, GuessEnemy},
		{anon, ong, GuessAlly}, // ally_of_everyone wins
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
		&Player{ID: "b", Color: config.Blue, Character: "USA", Signs: []string{"bs"}, Clues: []PlayerID{"w", "y"}},
		&Player{ID: "r", Color: config.Red, Character: "Russie", Signs: []string{"rs"}, Clues: []PlayerID{"w"}},
		&Player{ID: "w", Color: config.White, Character: "Inde", Signs: []string{"wb", "wr"}, Clues: []PlayerID{"b"}},
		&Player{ID: "y", Color: config.Yellow, Character: "ONG", Signs: []string{"ys"}, Clues: []PlayerID{"w"}},
	)

	tests := []struct {
		viewer PlayerID
		want   []Clue
	}{
		{"b", []Clue{{"Inde", []string{"wb"}}, {"ONG", []string{"ys"}}}}, // blue sees the white's blue sign
		{"r", []Clue{{"Inde", []string{"wr"}}}},                          // red sees the white's red sign
		{"w", []Clue{{"USA", []string{"bs"}}}},
		{"y", []Clue{{"Inde", []string{"wb", "wr"}}}}, // yellow sees both
	}
	for _, tt := range tests {
		got, err := g.Clues(tt.viewer)
		if err != nil {
			t.Fatalf("Clues(%s): %v", tt.viewer, err)
		}
		if !slices.EqualFunc(got, tt.want, func(a, b Clue) bool {
			return a.Character == b.Character && slices.Equal(a.Signs, b.Signs)
		}) {
			t.Errorf("Clues(%s) = %v, want %v", tt.viewer, got, tt.want)
		}
	}
}

func TestScore(t *testing.T) {
	g := handGame(
		&Player{ID: "b1", Color: config.Blue},
		&Player{ID: "b2", Color: config.Blue},
		&Player{ID: "r1", Color: config.Red},
		&Player{ID: "w1", Color: config.White, Choice: config.CampRed},
		&Player{ID: "w2", Color: config.White}, // stays neutral
		&Player{ID: "ong", Color: config.Yellow, Rule: config.AllyOfEveryone},
		&Player{ID: "anon", Color: config.Yellow, Rule: config.EnemyOfEveryone},
	)
	guesses := []struct {
		from, about PlayerID
		guess       Guess
	}{
		// b1: 3 right, 1 wrong
		{"b1", "b2", GuessAlly},
		{"b1", "r1", GuessEnemy},
		{"b1", "ong", GuessAlly},
		{"b1", "w2", GuessAlly}, // wrong: a neutral is an enemy
		// r1: 2 right, 2 wrong
		{"r1", "w1", GuessAlly},   // right: w1 joined red
		{"r1", "anon", GuessAlly}, // wrong
		{"r1", "b2", GuessAlly},   // wrong
		{"r1", "w2", GuessEnemy},  // right
		// w2: 2 right
		{"w2", "ong", GuessAlly},
		{"w2", "anon", GuessEnemy},
		// anon: 1 right, 1 wrong
		{"anon", "ong", GuessAlly},
		{"anon", "b1", GuessAlly},
	}
	for _, gs := range guesses {
		if err := g.SetGuess(gs.from, gs.about, gs.guess); err != nil {
			t.Fatalf("SetGuess(%s, %s): %v", gs.from, gs.about, err)
		}
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	_ = g.AddManualPoints("anon", 2, "piégé les USA")
	_ = g.AddManualPoints("b2", -1, "dénoncé")

	res := g.Score()

	wantPlayers := map[PlayerID]PlayerScore{
		"b1":   {Player: "b1", Camp: config.CampBlue, Right: 3, Wrong: 1, Total: 2},
		"b2":   {Player: "b2", Camp: config.CampBlue, Manual: -1, Total: -1},
		"r1":   {Player: "r1", Camp: config.CampRed, Right: 2, Wrong: 2, Total: 0},
		"w1":   {Player: "w1", Camp: config.CampRed},
		"w2":   {Player: "w2", Camp: config.CampNeutral, Right: 2, Total: 2},
		"ong":  {Player: "ong", Camp: config.CampNeutral},
		"anon": {Player: "anon", Camp: config.CampNeutral, Right: 1, Wrong: 1, Manual: 2, Total: 2},
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
		{Camp: config.CampBlue, Members: 2, Sum: 1, Score: 1},
		{Camp: config.CampRed, Members: 2, Sum: 0, Score: 0},
		{Camp: config.CampNeutral, Members: 3, Sum: 4, Score: 4 * 4.0 / 3}, // weighted by 4/n
	}
	if !slices.Equal(res.Teams, wantTeams) {
		t.Errorf("teams = %+v, want %+v", res.Teams, wantTeams)
	}
}

func TestScoreSkipsEmptyCamps(t *testing.T) {
	g := handGame(&Player{ID: "b", Color: config.Blue}, &Player{ID: "r", Color: config.Red})
	res := g.Score()
	if len(res.Teams) != 2 || res.Teams[0].Camp != config.CampBlue || res.Teams[1].Camp != config.CampRed {
		t.Errorf("teams = %+v, want blue and red only", res.Teams)
	}
}
