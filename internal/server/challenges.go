package server

import (
	"net/http"
	"strconv"

	"github.com/arthur91f/spyball/internal/game"
)

// challengeResult is a challenge shared by the traitors, done or not.
type challengeResult struct {
	Index int
	Text  string
	Done  bool
}

func challengeResults(g *game.Game) []challengeResult {
	var results []challengeResult
	for i, c := range g.Challenges {
		results = append(results, challengeResult{Index: i, Text: c.Text, Done: c.Done})
	}
	return results
}

// handleChallengesDone records which challenges were done, from the admin page once the game is closed:
// one checkbox "done" per challenge, whose value is its index.
func (s *Server) handleChallengesDone(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	err := s.store.Update(func(g *game.Game) error {
		done := make([]bool, len(g.Challenges))
		for _, v := range r.PostForm["done"] {
			i, err := strconv.Atoi(v)
			if err != nil || i < 0 || i >= len(done) {
				return game.ErrInvalid
			}
			done[i] = true
		}
		return g.SetChallengesDone(done)
	})
	if err != nil {
		s.render(w, http.StatusBadRequest, "admin", s.adminPage(s.describe(err), ""))
		return
	}
	redirect(w, r, "/admin#challenges")
}
