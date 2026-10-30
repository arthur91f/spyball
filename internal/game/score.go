package game

import "github.com/arthur91f/spyball/internal/config"

// The scoring rules, kept together so they are easy to revise.
const (
	// Points earned by the player who makes a guess.
	PointsAllyFound  = 2  // an ally guessed as an ally
	PointsEnemyFound = 2  // an enemy guessed as an enemy
	PointsWrongGuess = -2 // an ally guessed as an enemy, or an enemy as an ally

	// Points earned by the player a guess is about, whoever makes it and whether it is right or not.
	PointsDesignatedAlly  = 3  // guessed as an ally
	PointsDesignatedEnemy = -5 // guessed as an enemy

	// PointsChallenge is earned by the traitors' camp for each challenge done, whoever did it.
	PointsChallenge = 10
)

// PlayerScore details the score of one player.
type PlayerScore struct {
	Player  PlayerID
	Color   config.Color
	Traitor bool

	AlliesFound     int // allies the player guessed as allies
	EnemiesFound    int // enemies the player guessed as enemies
	Wrong           int // wrong guesses of the player
	DesignatedAlly  int // players who guessed the player as an ally
	DesignatedEnemy int // players who guessed the player as an enemy
	Manual          int // sum of the admin's manual points
	Total           int
}

// TeamScore is the score of a camp. A color camp scores the totals of its members who are not traitors.
// The traitors' camp scores the totals of all the traitors, plus the points of the challenges done.
type TeamScore struct {
	Color      config.Color // empty for the traitors' camp
	Traitors   bool         // the traitors' camp
	Members    int
	Challenges int // traitors' camp only: challenges done
	Score      int
}

// Results are the scores of every player (in game order) and of every camp that has members:
// the colors, then the traitors. Traitors are scored like everyone else, but for the traitors' camp.
type Results struct {
	Players []PlayerScore
	Teams   []TeamScore
}

// Score computes the results from the current guesses, challenges done and manual points.
func (g *Game) Score() Results {
	scores := make(map[PlayerID]*PlayerScore, len(g.Players))
	for _, p := range g.Players {
		scores[p.ID] = &PlayerScore{Player: p.ID, Color: p.Color, Traitor: p.Traitor}
	}
	for _, mp := range g.ManualPoints {
		if s := scores[mp.Player]; s != nil {
			s.Manual += mp.Points
		}
	}

	for _, p := range g.Players {
		s := scores[p.ID]
		for targetID, guess := range p.Guesses {
			target, err := g.Player(targetID)
			if err != nil {
				continue
			}
			switch {
			case guess != Relation(p, target):
				s.Wrong++
			case guess == GuessAlly:
				s.AlliesFound++
			default:
				s.EnemiesFound++
			}
			if guess == GuessAlly {
				scores[targetID].DesignatedAlly++
			} else {
				scores[targetID].DesignatedEnemy++
			}
		}
	}

	var res Results
	teams := map[config.Color]*TeamScore{}
	traitors := &TeamScore{Traitors: true}
	for _, p := range g.Players {
		s := scores[p.ID]
		s.Total = s.AlliesFound*PointsAllyFound + s.EnemiesFound*PointsEnemyFound + s.Wrong*PointsWrongGuess +
			s.DesignatedAlly*PointsDesignatedAlly + s.DesignatedEnemy*PointsDesignatedEnemy + s.Manual
		res.Players = append(res.Players, *s)

		team := teams[s.Color]
		if s.Traitor {
			team = traitors
		} else if team == nil {
			team = &TeamScore{Color: s.Color}
			teams[s.Color] = team
		}
		team.Members++
		team.Score += s.Total
	}

	for _, color := range config.Colors {
		if team := teams[color]; team != nil {
			res.Teams = append(res.Teams, *team)
		}
	}
	if traitors.Members > 0 {
		traitors.Challenges = g.ChallengesDone()
		traitors.Score += traitors.Challenges * PointsChallenge
		res.Teams = append(res.Teams, *traitors)
	}
	return res
}
