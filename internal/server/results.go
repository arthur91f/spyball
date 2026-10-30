package server

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/game"
)

// maxManualPoints bounds a single manual entry, to catch typos such as 10 instead of 1.
const maxManualPoints = 20

func (s *Server) registerResults(mux *http.ServeMux) {
	mux.HandleFunc("GET /results", s.handleResults)
	mux.Handle("POST /results/points", s.requireAdmin(s.handleAddPoints))
	mux.Handle("POST /results/points/{index}/delete", s.requireAdmin(s.handleRemovePoints))
}

type resultsPage struct {
	Closed  bool
	IsAdmin bool
	Error   string

	SetupName   string // the drawn setup, kept secret until the game is closed
	Teams       []teamResult
	TraitorName string         // themed name of a traitor
	Players     []playerResult // everyone, traitors included
	Challenges  []challengeResult
	Manual      []manualResult

	Points  pointsHelp
	Options []adminPlayer // admin form: who gets points
}

type teamResult struct {
	Color      config.Color // empty for the traitors' camp
	Traitors   bool
	Name       string
	Members    int
	Challenges int // traitors' camp only: challenges done
	Score      int
	Winner     bool
}

type playerResult struct {
	Rank      int
	Name      string
	Character string
	Color     config.Color
	TeamName  string
	Traitor   bool

	AlliesFound     int
	EnemiesFound    int
	Wrong           int
	DesignatedAlly  int
	DesignatedEnemy int
	Manual          string // signed, empty when zero
	Total           int
	Guesses         []guessResult
}

type guessResult struct {
	Name    string
	Guess   game.Guess
	Correct bool
}

type manualResult struct {
	Index  int
	Name   string
	Points string // signed
	Reason string
}

func (s *Server) resultsPage(r *http.Request, errMsg string) (resultsPage, error) {
	page := resultsPage{IsAdmin: s.isAdmin(r), Error: errMsg, Points: points}
	err := s.store.Read(func(g *game.Game) error {
		page.Closed = g.Phase == game.PhaseClosed
		if !page.Closed {
			return nil
		}
		theme := s.catalog.Themes[g.ThemeID]
		if theme == nil {
			return errors.New("theme " + g.ThemeID + " is no longer available")
		}
		res := g.Score()

		page.TraitorName = theme.Traitor.Name
		if setup := s.catalog.Setups[g.SetupID]; setup != nil {
			page.SetupName = setup.Name
		}

		best := 0
		for i, t := range res.Teams {
			if i == 0 || t.Score > best {
				best = t.Score
			}
		}
		for _, t := range res.Teams {
			tr := teamResult{
				Color:      t.Color,
				Traitors:   t.Traitors,
				Name:       theme.Teams[t.Color].Name,
				Members:    t.Members,
				Challenges: t.Challenges,
				Score:      t.Score,
				Winner:     t.Score == best,
			}
			if t.Traitors {
				tr.Name = theme.Traitor.Team
			}
			page.Teams = append(page.Teams, tr)
		}
		slices.SortStableFunc(page.Teams, func(a, b teamResult) int { return cmp.Compare(b.Score, a.Score) })

		for _, ps := range res.Players {
			p, err := g.Player(ps.Player)
			if err != nil {
				return err
			}
			pr := playerResult{
				Name:            p.Name,
				Character:       p.Character,
				Color:           p.Color,
				TeamName:        theme.Teams[p.Color].Name,
				Traitor:         p.Traitor,
				AlliesFound:     ps.AlliesFound,
				EnemiesFound:    ps.EnemiesFound,
				Wrong:           ps.Wrong,
				DesignatedAlly:  ps.DesignatedAlly,
				DesignatedEnemy: ps.DesignatedEnemy,
				Total:           ps.Total,
			}
			if ps.Manual != 0 {
				pr.Manual = fmt.Sprintf("%+d", ps.Manual)
			}
			for _, other := range g.Players {
				guess, ok := p.Guesses[other.ID]
				if !ok {
					continue
				}
				pr.Guesses = append(pr.Guesses, guessResult{Name: other.Name, Guess: guess, Correct: guess == game.Relation(p, other)})
			}
			page.Players = append(page.Players, pr)
			page.Options = append(page.Options, adminPlayer{ID: p.ID, Name: p.Name})
		}
		rank(page.Players)
		slices.SortFunc(page.Options, func(a, b adminPlayer) int {
			return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})

		page.Challenges = challengeResults(g)

		for i, mp := range g.ManualPoints {
			p, err := g.Player(mp.Player)
			if err != nil {
				return err
			}
			page.Manual = append(page.Manual, manualResult{Index: i, Name: p.Name, Points: fmt.Sprintf("%+d", mp.Points), Reason: mp.Reason})
		}
		return nil
	})
	return page, err
}

// rank sorts players by total, then name, and numbers them; ties share their rank.
func rank(players []playerResult) {
	slices.SortStableFunc(players, func(a, b playerResult) int {
		return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)))
	})
	for i := range players {
		players[i].Rank = i + 1
		if i > 0 && players[i].Total == players[i-1].Total {
			players[i].Rank = players[i-1].Rank
		}
	}
}

func (s *Server) renderResults(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	page, err := s.resultsPage(r, errMsg)
	if err != nil {
		s.render(w, http.StatusInternalServerError, "error", errorPage{Message: s.describe(err)})
		return
	}
	s.render(w, status, "results", page)
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	s.renderResults(w, r, http.StatusOK, "")
}

func (s *Server) handleAddPoints(w http.ResponseWriter, r *http.Request) {
	id := game.PlayerID(r.PostFormValue("player"))
	raw := r.PostFormValue("points")
	points, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(raw), "+"))
	if err != nil || points < -maxManualPoints || points > maxManualPoints {
		s.renderResults(w, r, http.StatusBadRequest, s.describe(fmt.Errorf("%w: points %q", game.ErrInvalid, raw)))
		return
	}

	err = s.store.Update(func(g *game.Game) error { return g.AddManualPoints(id, points, r.PostFormValue("reason")) })
	if err != nil {
		s.renderResults(w, r, http.StatusBadRequest, s.describe(err))
		return
	}
	redirect(w, r, "/results#manual")
}

func (s *Server) handleRemovePoints(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.PathValue("index"))
	if err == nil {
		err = s.store.Update(func(g *game.Game) error { return g.RemoveManualPoints(index) })
	} else {
		err = game.ErrInvalid
	}
	if err != nil {
		s.renderResults(w, r, http.StatusBadRequest, s.describe(err))
		return
	}
	redirect(w, r, "/results#manual")
}
