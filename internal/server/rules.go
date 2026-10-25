package server

import (
	"cmp"
	"net/http"
	"slices"
	"strings"

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

	Blue, Red, Neutral config.Team
	AllyOfEveryone     string // names of the yellow characters with each rule, if any
	EnemyOfEveryone    string
	NeutralFactor      int

	// Once roles are drawn: the characters in play, by color. Never who plays them.
	SetupName string
	Cast      []castGroup
}

type castGroup struct {
	Color config.Color
	Label string
	Names []string
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	var gameTheme, gameSetup string
	_ = s.store.Read(func(g *game.Game) error {
		gameTheme, gameSetup = g.ThemeID, g.SetupID
		return nil
	})

	page := rulesPage{NeutralFactor: game.NeutralTeamFactor}
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

	page.Blue, page.Red, page.Neutral = t.Teams[config.CampBlue], t.Teams[config.CampRed], t.Teams[config.CampNeutral]
	page.AllyOfEveryone = yellowNames(t, config.AllyOfEveryone)
	page.EnemyOfEveryone = yellowNames(t, config.EnemyOfEveryone)

	if setup := s.catalog.Setups[gameSetup]; setup != nil && t.ID == gameTheme {
		if cast, err := t.Cast(setup); err == nil {
			page.SetupName = setup.Name
			page.Cast = s.castGroups(t, setup, cast)
		}
	}

	s.render(w, http.StatusOK, "rules", page)
}

func yellowNames(t *config.Theme, rule config.Rule) string {
	var names []string
	for _, c := range t.Characters[config.Yellow] {
		if c.Rule == rule {
			names = append(names, c.Name)
		}
	}
	return strings.Join(names, ", ")
}

func (s *Server) castGroups(t *config.Theme, setup *config.Setup, cast map[config.SlotID]config.Character) []castGroup {
	var groups []castGroup
	for _, color := range config.Colors {
		var names []string
		for _, slot := range setup.Slots {
			if slot.Color == color {
				names = append(names, cast[slot.ID].Name)
			}
		}
		if len(names) == 0 {
			continue
		}
		label := t.Teams[config.CampNeutral].Name
		switch color {
		case config.Blue:
			label = t.Teams[config.CampBlue].Name
		case config.Red:
			label = t.Teams[config.CampRed].Name
		case config.White:
			label, _ = s.locale.T("rules.cast.white")
		case config.Yellow:
			label, _ = s.locale.T("rules.cast.yellow")
		}
		groups = append(groups, castGroup{Color: color, Label: label, Names: names})
	}
	return groups
}
