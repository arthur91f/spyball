package game

import (
	"errors"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/arthur91f/spyball"
	"github.com/arthur91f/spyball/internal/config"
)

func loadContent(t *testing.T) (*config.Theme, *config.Setup) {
	t.Helper()
	cat, err := config.Load(spyball.Content)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cat.Themes["ball-of-nations"], cat.Setups["12-classic"]
}

func newGameWithPlayers(t *testing.T, n int) *Game {
	t.Helper()
	g := New()
	for i := range n {
		if _, err := g.AddPlayer("Guest " + strconv.Itoa(i+1)); err != nil {
			t.Fatalf("AddPlayer: %v", err)
		}
	}
	return g
}

func drawnGame(t *testing.T, seed uint64) *Game {
	t.Helper()
	theme, setup := loadContent(t)
	g := newGameWithPlayers(t, setup.PlayerCount())
	if err := g.Draw(theme, setup, rand.New(rand.NewPCG(seed, 0))); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	return g
}

func TestAddPlayer(t *testing.T) {
	g := New()
	alice, err := g.AddPlayer("  Alice   Martin ")
	if err != nil {
		t.Fatalf("AddPlayer: %v", err)
	}
	if alice.Name != "Alice Martin" || alice.ID != "p1" {
		t.Errorf("player = %+v, want name %q and id p1", alice, "Alice Martin")
	}
	if _, err := g.AddPlayer("alice martin"); !errors.Is(err, ErrDuplicateName) {
		t.Error("duplicate name (other case): want error")
	}
	if _, err := g.AddPlayer("   "); !errors.Is(err, ErrEmptyName) {
		t.Error("empty name: want error")
	}

	bob, _ := g.AddPlayer("Bob")
	if err := g.RemovePlayer(alice.ID); err != nil {
		t.Fatalf("RemovePlayer: %v", err)
	}
	carol, _ := g.AddPlayer("Carol")
	if carol.ID == alice.ID || carol.ID == bob.ID {
		t.Errorf("ID %s reused", carol.ID)
	}
	if got := len(g.Players); got != 2 {
		t.Errorf("len(Players) = %d, want 2", got)
	}
}

func TestPhaseGuards(t *testing.T) {
	g := newGameWithPlayers(t, 2)
	p1, p2 := g.Players[0].ID, g.Players[1].ID

	if err := g.SetGuess(p1, p2, GuessAlly); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("SetGuess during setup: err = %v, want ErrWrongPhase", err)
	}
	if err := g.Close(); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("Close during setup: err = %v, want ErrWrongPhase", err)
	}

	g = drawnGame(t, 1)
	p1, p2 = g.Players[0].ID, g.Players[1].ID
	if _, err := g.AddPlayer("Late"); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("AddPlayer while running: err = %v, want ErrWrongPhase", err)
	}
	if err := g.AddManualPoints(p1, 1, "défi"); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("AddManualPoints while running: err = %v, want ErrWrongPhase", err)
	}
	if err := g.SetGuess(p1, p2, GuessAlly); err != nil {
		t.Errorf("SetGuess while running: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := g.SetGuess(p1, p2, GuessEnemy); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("SetGuess after close: err = %v, want ErrWrongPhase", err)
	}
	if err := g.AddManualPoints(p1, 2, "défi"); err != nil {
		t.Errorf("AddManualPoints after close: %v", err)
	}
}

func TestSetGuess(t *testing.T) {
	g := drawnGame(t, 1)
	p1, p2 := g.Players[0], g.Players[1]

	if err := g.SetGuess(p1.ID, p1.ID, GuessAlly); err == nil {
		t.Error("guess about yourself: want error")
	}
	if err := g.SetGuess(p1.ID, "nobody", GuessAlly); !errors.Is(err, ErrUnknownPlayer) {
		t.Errorf("guess about unknown player: err = %v, want ErrUnknownPlayer", err)
	}
	if err := g.SetGuess(p1.ID, p2.ID, "maybe"); err == nil {
		t.Error("unknown guess: want error")
	}

	_ = g.SetGuess(p1.ID, p2.ID, GuessAlly)
	_ = g.SetGuess(p1.ID, p2.ID, GuessEnemy)
	if got := p1.Guesses[p2.ID]; got != GuessEnemy {
		t.Errorf("guess = %q, want enemy (changed)", got)
	}
	_ = g.SetGuess(p1.ID, p2.ID, GuessNone)
	if _, ok := p1.Guesses[p2.ID]; ok {
		t.Error("GuessNone did not clear the guess")
	}
}

func TestChooseCamp(t *testing.T) {
	g := drawnGame(t, 1)
	var white, blue *Player
	for _, p := range g.Players {
		switch p.Color {
		case config.White:
			white = p
		case config.Blue:
			blue = p
		}
	}

	if white.Camp() != config.CampNeutral {
		t.Errorf("white without choice: camp = %s, want neutral", white.Camp())
	}
	if err := g.ChooseCamp(white.ID, config.CampRed); err != nil {
		t.Fatalf("ChooseCamp: %v", err)
	}
	if err := g.ChooseCamp(white.ID, config.CampBlue); err != nil {
		t.Fatalf("ChooseCamp again: %v", err)
	}
	if white.Camp() != config.CampBlue {
		t.Errorf("camp = %s, want blue", white.Camp())
	}
	if err := g.ChooseCamp(blue.ID, config.CampRed); err == nil {
		t.Error("blue player choosing a camp: want error")
	}
	if err := g.ChooseCamp(white.ID, "green"); err == nil {
		t.Error("unknown camp: want error")
	}
}

func TestDraw(t *testing.T) {
	theme, setup := loadContent(t)
	g := drawnGame(t, 42)

	if g.Phase != PhaseRunning || g.ThemeID != theme.ID || g.SetupID != setup.ID {
		t.Fatalf("after draw: phase %s, theme %s, setup %s", g.Phase, g.ThemeID, g.SetupID)
	}

	slots := map[config.SlotID]*Player{}
	signs := map[string]bool{}
	for _, p := range g.Players {
		if slots[p.Slot] != nil {
			t.Errorf("slot %s given twice", p.Slot)
		}
		slots[p.Slot] = p

		slot, ok := setup.Slot(p.Slot)
		if !ok || slot.Color != p.Color || slot.Rule != p.Rule {
			t.Errorf("player %s: slot %s does not match color %s / rule %s", p.Name, p.Slot, p.Color, p.Rule)
		}
		if p.Character == "" {
			t.Errorf("player %s: no character", p.Name)
		}

		wantColors := p.Color.SignColors()
		if len(p.Signs) != len(wantColors) {
			t.Fatalf("player %s (%s): %d signs, want %d", p.Name, p.Color, len(p.Signs), len(wantColors))
		}
		for i, sign := range p.Signs {
			if !slices.Contains(theme.Signs[wantColors[i]], sign) {
				t.Errorf("player %s: sign %q is not a %s sign", p.Name, sign, wantColors[i])
			}
			if signs[sign] {
				t.Errorf("sign %q drawn twice", sign)
			}
			signs[sign] = true
		}
	}
	if len(slots) != setup.PlayerCount() {
		t.Errorf("%d slots assigned, want %d", len(slots), setup.PlayerCount())
	}

	for _, slot := range setup.Slots {
		p := slots[slot.ID]
		var got []config.SlotID
		for _, id := range p.Clues {
			target, _ := g.Player(id)
			got = append(got, target.Slot)
		}
		if !slices.Equal(got, slot.Clues) {
			t.Errorf("slot %s: clues %v, want %v", slot.ID, got, slot.Clues)
		}
	}
	if slots["blue-1"].Character != "USA" || slots["yellow-2"].Character != "Anonymous" {
		t.Errorf("characters not cast by slot: blue-1 %q, yellow-2 %q", slots["blue-1"].Character, slots["yellow-2"].Character)
	}
}

func TestDrawIsRandomButReproducible(t *testing.T) {
	assignment := func(g *Game) []string {
		var s []string
		for _, p := range g.Players {
			s = append(s, string(p.Slot)+":"+strings.Join(p.Signs, "|"))
		}
		return s
	}
	if !slices.Equal(assignment(drawnGame(t, 7)), assignment(drawnGame(t, 7))) {
		t.Error("same seed gave different draws")
	}
	if slices.Equal(assignment(drawnGame(t, 7)), assignment(drawnGame(t, 8))) {
		t.Error("different seeds gave the same draw")
	}
}

func TestDrawNeedsExactPlayerCount(t *testing.T) {
	theme, setup := loadContent(t)
	g := newGameWithPlayers(t, setup.PlayerCount()-1)
	if err := g.Draw(theme, setup, rand.New(rand.NewPCG(1, 0))); !errors.Is(err, ErrPlayerCount) {
		t.Fatal("draw with 11 players on a 12-player setup: want error")
	}
	if g.Phase != PhaseSetup {
		t.Errorf("phase = %s after failed draw, want setup", g.Phase)
	}
}

func TestCancelDraw(t *testing.T) {
	g := drawnGame(t, 1)
	_ = g.SetGuess(g.Players[0].ID, g.Players[1].ID, GuessAlly)

	if err := g.CancelDraw(); err != nil {
		t.Fatalf("CancelDraw: %v", err)
	}
	if g.Phase != PhaseSetup || g.SetupID != "" {
		t.Errorf("phase %s, setup %q after cancel", g.Phase, g.SetupID)
	}
	for _, p := range g.Players {
		if p.Slot != "" || p.Character != "" || len(p.Signs) > 0 || len(p.Guesses) > 0 || p.Name == "" {
			t.Errorf("player not reset: %+v", p)
		}
	}
}

func TestLogin(t *testing.T) {
	g := newGameWithPlayers(t, 1)
	if _, err := g.Login("Guest 1", ""); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("login before the draw: err = %v, want ErrWrongPhase", err)
	}

	g = drawnGame(t, 1)
	p := g.Players[0]

	if _, err := g.Login("Nobody", ""); !errors.Is(err, ErrUnknownPlayer) {
		t.Errorf("unknown name: err = %v, want ErrUnknownPlayer", err)
	}

	// First login: the name is enough, the character gets revealed.
	got, err := g.Login(" guest 1 ", "")
	if err != nil || got != p || !p.Revealed {
		t.Fatalf("first login: player %v, err %v, revealed %v", got, err, p.Revealed)
	}

	// Next logins need the character.
	if _, err := g.Login("Guest 1", ""); !errors.Is(err, ErrCharacterRequired) {
		t.Errorf("second login without character: err = %v, want ErrCharacterRequired", err)
	}
	if _, err := g.Login("Guest 1", "Atlantide"); !errors.Is(err, ErrWrongCharacter) {
		t.Errorf("wrong character: err = %v, want ErrWrongCharacter", err)
	}
	if _, err := g.Login("Guest 1", "  "+strings.ToUpper(p.Character)+" "); err != nil {
		t.Errorf("right character, other case: %v", err)
	}

	// The admin can reset a forgotten character.
	if err := g.ResetLogin(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Login("Guest 1", ""); err != nil {
		t.Errorf("login after reset: %v", err)
	}

	// Logging in still works once the game is closed, to look at your own page.
	_ = g.Close()
	if _, err := g.Login("Guest 1", p.Character); err != nil {
		t.Errorf("login after close: %v", err)
	}
	if err := g.ResetLogin(p.ID); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("reset login after close: err = %v, want ErrWrongPhase", err)
	}
}
