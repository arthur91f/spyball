package game

import "github.com/arthur91f/spyball/internal/config"

// The scoring rules of the V1, kept together so they are easy to revise.
const (
	pointsRightGuess = 1
	pointsWrongGuess = -1
	// The neutral team score is multiplied by neutralTeamFactor/n, n being its number of members,
	// to make up for being fewer.
	neutralTeamFactor = 4.0
)

// PlayerScore details the score of one player.
type PlayerScore struct {
	Player PlayerID
	Camp   config.Camp
	Right  int // correct guesses
	Wrong  int // wrong guesses
	Manual int // sum of the admin's manual points
	Total  int
}

// TeamScore is the score of a camp: the sum of its members' totals, weighted for the neutral team.
type TeamScore struct {
	Camp    config.Camp
	Members int
	Sum     int
	Score   float64
}

// Results are the scores of every player (in game order) and of every camp that has members.
type Results struct {
	Players []PlayerScore
	Teams   []TeamScore
}

// Score computes the results from the current guesses, camp choices and manual points.
func (g *Game) Score() Results {
	manual := map[PlayerID]int{}
	for _, mp := range g.ManualPoints {
		manual[mp.Player] += mp.Points
	}

	var res Results
	teams := map[config.Camp]*TeamScore{}
	for _, p := range g.Players {
		s := PlayerScore{Player: p.ID, Camp: p.Camp(), Manual: manual[p.ID]}
		for targetID, guess := range p.Guesses {
			target, err := g.Player(targetID)
			if err != nil {
				continue
			}
			if guess == Relation(p, target) {
				s.Right++
			} else {
				s.Wrong++
			}
		}
		s.Total = s.Right*pointsRightGuess + s.Wrong*pointsWrongGuess + s.Manual
		res.Players = append(res.Players, s)

		team := teams[s.Camp]
		if team == nil {
			team = &TeamScore{Camp: s.Camp}
			teams[s.Camp] = team
		}
		team.Members++
		team.Sum += s.Total
	}

	for _, camp := range config.Camps {
		team := teams[camp]
		if team == nil {
			continue
		}
		team.Score = float64(team.Sum)
		if camp == config.CampNeutral {
			team.Score *= neutralTeamFactor / float64(team.Members)
		}
		res.Teams = append(res.Teams, *team)
	}
	return res
}
