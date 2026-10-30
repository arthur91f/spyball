package server

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/game"
)

func (s *Server) registerRules(mux *http.ServeMux) {
	mux.HandleFunc("GET /rules", s.handleRules)
}

// rulesPage is the generic rules (from the locale) dressed with the names of a theme.
type rulesPage struct {
	Theme  *config.Theme
	Themes []*config.Theme // to read the rules of another theme

	Blue, Red config.Team
	Traitor   config.Role
	Points    pointsHelp
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	var gameTheme string
	_ = s.store.Read(func(g *game.Game) error {
		gameTheme = g.ThemeID
		return nil
	})

	page := rulesPage{Points: points}
	for _, t := range s.catalog.Themes {
		page.Themes = append(page.Themes, t)
	}
	slices.SortFunc(page.Themes, func(a, b *config.Theme) int { return cmp.Compare(a.Name, b.Name) })

	// The theme asked for, else the one of the current game, else the first one.
	page.Theme = s.catalog.Themes[r.URL.Query().Get("theme")]
	if page.Theme == nil {
		page.Theme = s.catalog.Themes[gameTheme]
	}
	if page.Theme == nil {
		page.Theme = page.Themes[0]
	}
	t := page.Theme

	page.Blue, page.Red, page.Traitor = t.Teams[config.Blue], t.Teams[config.Red], t.Traitor

	s.render(w, http.StatusOK, "rules", page)
}

// textTheme makes the page use the texts of the theme shown, which may not be the one of the game.
func (p rulesPage) textTheme() *config.Theme { return p.Theme }
