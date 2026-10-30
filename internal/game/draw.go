package game

import (
	"fmt"
	"math/rand/v2"

	"github.com/arthur91f/spyball/internal/config"
)

// Draw randomly assigns the setup slots to the players, gives each one its themed character,
// draws signs and challenges without duplicates and resolves clues. The game moves to the running phase.
func (g *Game) Draw(theme *config.Theme, setup *config.Setup, rng *rand.Rand) error {
	if err := g.requirePhase(PhaseSetup); err != nil {
		return err
	}
	if len(g.Players) != setup.PlayerCount() {
		return fmt.Errorf("%w: setup %s is for %d players, game has %d", ErrPlayerCount, setup.ID, setup.PlayerCount(), len(g.Players))
	}
	cast, err := theme.Cast(setup)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrIncompatibleTheme, err)
	}

	// Shuffled copies of the sign lists, consumed from the end.
	pools := map[config.Color][]string{}
	for _, color := range config.Colors { // fixed order: map iteration would make the draw irreproducible
		pools[color] = shuffled(theme.Signs[color], rng)
	}
	// The challenges are the same for every traitor.
	g.Challenges = nil
	for _, text := range shuffled(theme.Challenges, rng)[:setup.Challenges] {
		g.Challenges = append(g.Challenges, Challenge{Text: text})
	}

	// Player i gets slot perm[i].
	perm := rng.Perm(len(g.Players))
	bySlot := make(map[config.SlotID]*Player, len(g.Players))
	for i, p := range g.Players {
		slot := setup.Slots[perm[i]]
		char := cast[slot.ID]
		*p = Player{
			ID:          p.ID,
			Name:        p.Name,
			Slot:        slot.ID,
			Color:       slot.Color,
			Traitor:     slot.Traitor,
			Character:   char.Name,
			Description: char.Description,
		}
		bySlot[slot.ID] = p
	}

	// Signs, challenges and clues in setup order, so the result only depends on the rng.
	for _, slot := range setup.Slots {
		p := bySlot[slot.ID]
		pool := pools[slot.Color]
		p.Sign, pools[slot.Color] = pool[len(pool)-1], pool[:len(pool)-1]
		for _, clue := range slot.Clues {
			p.Clues = append(p.Clues, bySlot[clue].ID)
		}
	}

	g.ThemeID = theme.ID
	g.SetupID = setup.ID
	g.Phase = PhaseRunning
	return nil
}

func shuffled(list []string, rng *rand.Rand) []string {
	list = append([]string(nil), list...)
	rng.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	return list
}

// CancelDraw forgets all roles, guesses and challenges and goes back to the setup phase,
// e.g. to fix the player list. Players are kept.
func (g *Game) CancelDraw() error {
	if err := g.requirePhase(PhaseRunning); err != nil {
		return err
	}
	for _, p := range g.Players {
		*p = Player{ID: p.ID, Name: p.Name}
	}
	g.ThemeID, g.SetupID, g.Challenges = "", "", nil
	g.Phase = PhaseSetup
	return nil
}
