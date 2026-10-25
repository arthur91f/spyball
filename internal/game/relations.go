package game

import (
	"slices"

	"github.com/arthur91f/spyball/internal/config"
)

// Camp is the camp the player scores with: their color for blue and red, their choice for white
// (neutral if they made none), neutral for yellow.
func (p *Player) Camp() config.Camp {
	switch p.Color {
	case config.Blue:
		return config.CampBlue
	case config.Red:
		return config.CampRed
	case config.White:
		if p.Choice != "" {
			return p.Choice
		}
	}
	return config.CampNeutral
}

// Relation tells whether b is an ally or an enemy of a. It is symmetric:
//   - anyone paired with an ally_of_everyone player is an ally (this wins over enemy_of_everyone);
//   - otherwise anyone paired with an enemy_of_everyone player is an enemy;
//   - otherwise players of the same camp are allies and players of different camps are enemies,
//     so a neutral is an enemy of blue and red players and an ally of the other neutrals.
func Relation(a, b *Player) Guess {
	switch {
	case a.Rule == config.AllyOfEveryone || b.Rule == config.AllyOfEveryone:
		return GuessAlly
	case a.Rule == config.EnemyOfEveryone || b.Rule == config.EnemyOfEveryone:
		return GuessEnemy
	case a.Camp() == b.Camp():
		return GuessAlly
	default:
		return GuessEnemy
	}
}

// Clue is what a player knows about another player: their character and the sign(s) to spot them.
// It never reveals the guest's name.
type Clue struct {
	Character string
	Signs     []string
}

// Clues returns the clues of a player. About a white player, a blue sees only the blue sign,
// a red only the red sign, and white or yellow players see both.
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
		clues = append(clues, Clue{Character: target.Character, Signs: target.signsSeenBy(p.Color)})
	}
	return clues, nil
}

func (p *Player) signsSeenBy(viewer config.Color) []string {
	colors := p.Color.SignColors()
	if len(colors) > 1 {
		if i := slices.Index(colors, viewer); i >= 0 {
			return []string{p.Signs[i]}
		}
	}
	return p.Signs
}
