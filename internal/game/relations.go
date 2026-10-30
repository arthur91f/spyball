package game

import "github.com/arthur91f/spyball/internal/config"

// Relation tells whether b is an ally or an enemy of a: players of the same color are allies,
// players of different colors are enemies. Traitors are ordinary members of their color.
func Relation(a, b *Player) Guess {
	if a.Color == b.Color {
		return GuessAlly
	}
	return GuessEnemy
}

// Clue is what a player knows about another player: their color and their sign. It reveals neither
// the guest's name, nor their character (which is their password), nor whether they are a traitor.
type Clue struct {
	Color config.Color
	Sign  string
}

// Clues returns the clues of a player.
func (g *Game) Clues(id PlayerID) ([]Clue, error) {
	p, err := g.Player(id)
	if err != nil {
		return nil, err
	}

	clues := make([]Clue, 0, len(p.Clues))
	for _, targetID := range p.Clues {
		target, err := g.Player(targetID)
		if err != nil {
			return nil, err
		}
		clues = append(clues, Clue{Color: target.Color, Sign: target.Sign})
	}
	return clues, nil
}
