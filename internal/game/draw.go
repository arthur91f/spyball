package game

import (
	"fmt"
	"math/rand/v2"

	"github.com/arthur91f/spyball/internal/config"
)

// Draw randomly assigns the setup slots to the players, gives each one its themed character,
// draws signs without duplicates and resolves clues. The game moves to the running phase.
func (g *Game) Draw(theme *config.Theme, setup *config.Setup, rng *rand.Rand) error {
	if err := g.requirePhase(PhaseSetup); err != nil {
		return err
	}
	if len(g.Players) != setup.PlayerCount() {
		return fmt.Errorf("setup %s is for %d players, game has %d", setup.ID, setup.PlayerCount(), len(g.Players))
	}
	cast, err := theme.Cast(setup)
	if err != nil {
		return err
	}

	// Shuffled copies of the sign lists, consumed from the end.
	pools := map[config.Color][]string{}
	for color, signs := range theme.Signs {
		pool := append([]string(nil), signs...)
		rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		pools[color] = pool
	}
	drawSign := func(c config.Color) string {
		pool := pools[c]
		sign := pool[len(pool)-1]
		pools[c] = pool[:len(pool)-1]
		return sign
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
			Rule:        slot.Rule,
			Character:   char.Name,
			Description: char.Description,
		}
		bySlot[slot.ID] = p
	}

	// Signs and clues in setup order, so the result only depends on the rng.
	for _, slot := range setup.Slots {
		p := bySlot[slot.ID]
		for _, c := range slot.Color.SignColors() {
			p.Signs = append(p.Signs, drawSign(c))
		}
		for _, clue := range slot.Clues {
			p.Clues = append(p.Clues, bySlot[clue].ID)
		}
	}

	g.ThemeID = theme.ID
	g.SetupID = setup.ID
	g.Phase = PhaseRunning
	return nil
}

// CancelDraw forgets all roles, guesses and choices and goes back to the setup phase,
// e.g. to fix the player list. Players are kept.
func (g *Game) CancelDraw() error {
	if err := g.requirePhase(PhaseRunning); err != nil {
		return err
	}
	for _, p := range g.Players {
		*p = Player{ID: p.ID, Name: p.Name}
	}
	g.ThemeID, g.SetupID = "", ""
	g.Phase = PhaseSetup
	return nil
}
