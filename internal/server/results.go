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

	Teams   []teamResult
	Players []playerResult
	Manual  []manualResult

	NeutralFactor int
	Options       []adminPlayer // admin form: who gets points
}

type teamResult struct {
	Camp     config.Camp
	Name     string
	Members  int
	Sum      int
	Score    string // formatted, weighted for the neutral team
	Weighted bool
	Winner   bool
	score    float64
}

type playerResult struct {
	Rank      int
	Name      string
	Character string
	Color     config.Color
	Rule      config.Rule
	Camp      config.Camp
	CampName  string
	Right     int
	Wrong     int
	Manual    string // signed, empty when zero
	Total     int
	Guesses   []guessResult
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
	page := resultsPage{IsAdmin: s.isAdmin(r), Error: errMsg, NeutralFactor: game.NeutralTeamFactor}
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

		best := 0.0
		for i, t := range res.Teams {
			if i == 0 || t.Score > best {
				best = t.Score
			}
		}
		for _, t := range res.Teams {
			page.Teams = append(page.Teams, teamResult{
				Camp:     t.Camp,
				Name:     theme.Teams[t.Camp].Name,
				Members:  t.Members,
				Sum:      t.Sum,
				Score:    s.formatScore(t.Score),
				Weighted: t.Camp == config.CampNeutral,
				Winner:   t.Score == best,
				score:    t.Score,
			})
		}
		slices.SortStableFunc(page.Teams, func(a, b teamResult) int { return cmp.Compare(b.score, a.score) })

		for _, ps := range res.Players {
			p, err := g.Player(ps.Player)
			if err != nil {
				return err
			}
			pr := playerResult{
				Name:      p.Name,
				Character: p.Character,
				Color:     p.Color,
				Rule:      p.Rule,
				Camp:      ps.Camp,
				CampName:  theme.Teams[ps.Camp].Name,
				Right:     ps.Right,
				Wrong:     ps.Wrong,
				Total:     ps.Total,
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
		slices.SortStableFunc(page.Players, func(a, b playerResult) int {
			return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)))
		})
		for i := range page.Players {
			page.Players[i].Rank = i + 1
			if i > 0 && page.Players[i].Total == page.Players[i-1].Total {
				page.Players[i].Rank = page.Players[i-1].Rank // ties share their rank
			}
		}
		slices.SortFunc(page.Options, func(a, b adminPlayer) int {
			return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})

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

// formatScore shows one decimal only when needed, with the locale's decimal separator.
func (s *Server) formatScore(score float64) string {
	text := strconv.FormatFloat(score, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	if sep, err := s.locale.T("format.decimal_separator"); err == nil {
		text = strings.Replace(text, ".", sep, 1)
	}
	return text
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
