package server

import (
	"cmp"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"slices"
	"strings"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/game"
)

// The admin page never shows roles, signs nor guesses: the admin may be playing too.

func (s *Server) registerAdmin(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/login", s.handleAdminLoginForm)
	mux.HandleFunc("POST /admin/login", s.handleAdminLogin)
	mux.HandleFunc("POST /admin/logout", s.handleAdminLogout)

	mux.Handle("GET /admin", s.requireAdmin(s.handleAdmin))
	mux.Handle("POST /admin/players", s.requireAdmin(s.handleAddPlayers))
	mux.Handle("POST /admin/players/{id}/delete", s.requireAdmin(s.handleRemovePlayer))
	mux.Handle("POST /admin/draw", s.requireAdmin(s.handleDraw))
	mux.Handle("POST /admin/cancel-draw", s.requireAdmin(s.handleCancelDraw))
	mux.Handle("POST /admin/close", s.requireAdmin(s.handleClose))
	mux.Handle("POST /admin/reset", s.requireAdmin(s.handleReset))
}

// adminToken is stored in the admin cookie. It depends on the password, so changing the password
// logs every admin out.
func (s *Server) adminToken() string {
	sum := sha256.Sum256([]byte("admin\x00" + s.opts.AdminPassword))
	return hex.EncodeToString(sum[:16])
}

func (s *Server) isAdmin(r *http.Request) bool {
	v, ok := s.sessions.get(r, adminCookie)
	return ok && subtle.ConstantTimeCompare([]byte(v), []byte(s.adminToken())) == 1
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isAdmin(r) {
			redirect(w, r, "/admin/login")
			return
		}
		next(w, r)
	})
}

type adminLoginPage struct {
	Error string
}

func (s *Server) handleAdminLoginForm(w http.ResponseWriter, r *http.Request) {
	if s.isAdmin(r) {
		redirect(w, r, "/admin")
		return
	}
	s.render(w, http.StatusOK, "admin_login", adminLoginPage{})
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	got := sha256.Sum256([]byte(r.PostFormValue("password")))
	want := sha256.Sum256([]byte(s.opts.AdminPassword))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		msg, _ := s.locale.T("admin.login.wrong_password")
		s.render(w, http.StatusUnauthorized, "admin_login", adminLoginPage{Error: msg})
		return
	}
	s.sessions.set(w, adminCookie, s.adminToken())
	redirect(w, r, "/admin")
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	s.sessions.clear(w, adminCookie)
	redirect(w, r, "/")
}

type adminPage struct {
	Error      string
	NamesInput string // kept on error so the admin does not retype the list

	Phase   game.Phase
	Players []adminPlayer

	// Setup phase: what can be drawn.
	Themes        []*config.Theme
	SelectedTheme string
	Setups        []*config.Setup // setups for exactly len(Players) players
	AllSetups     []*config.Setup // every setup, by player count, as a hint

	// Running and closed phases.
	ThemeName string
	SetupName string
}

type adminPlayer struct {
	ID   game.PlayerID
	Name string
}

func (s *Server) adminPage(errMsg, namesInput string) adminPage {
	page := adminPage{Error: errMsg, NamesInput: namesInput}
	_ = s.store.Read(func(g *game.Game) error {
		page.Phase = g.Phase
		for _, p := range g.Players {
			page.Players = append(page.Players, adminPlayer{ID: p.ID, Name: p.Name})
		}
		page.SelectedTheme = g.ThemeID
		if t := s.catalog.Themes[g.ThemeID]; t != nil {
			page.ThemeName = t.Name
		}
		if st := s.catalog.Setups[g.SetupID]; st != nil {
			page.SetupName = st.Name
		}
		return nil
	})

	for _, t := range s.catalog.Themes {
		page.Themes = append(page.Themes, t)
	}
	slices.SortFunc(page.Themes, func(a, b *config.Theme) int { return cmp.Compare(a.Name, b.Name) })
	if page.SelectedTheme == "" && len(page.Themes) > 0 {
		page.SelectedTheme = page.Themes[0].ID
	}

	for _, st := range s.catalog.Setups {
		page.AllSetups = append(page.AllSetups, st)
		if st.PlayerCount() == len(page.Players) {
			page.Setups = append(page.Setups, st)
		}
	}
	bySize := func(a, b *config.Setup) int {
		return cmp.Or(cmp.Compare(a.PlayerCount(), b.PlayerCount()), cmp.Compare(a.Name, b.Name))
	}
	slices.SortFunc(page.AllSetups, bySize)
	slices.SortFunc(page.Setups, bySize)
	return page
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "admin", s.adminPage("", ""))
}

// adminUpdate applies fn to the game and redirects to the admin page, or shows the page with the error.
func (s *Server) adminUpdate(w http.ResponseWriter, r *http.Request, fn func(g *game.Game) error) {
	if err := s.store.Update(fn); err != nil {
		s.render(w, http.StatusBadRequest, "admin", s.adminPage(s.describe(err), ""))
		return
	}
	redirect(w, r, "/admin")
}

// handleAddPlayers adds one player per non-empty line. All names are added or none.
func (s *Server) handleAddPlayers(w http.ResponseWriter, r *http.Request) {
	input := r.PostFormValue("names")
	var failed string
	err := s.store.Update(func(g *game.Game) error {
		for line := range strings.Lines(input) {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if _, err := g.AddPlayer(line); err != nil {
				failed = strings.TrimSpace(line)
				return err
			}
		}
		return nil
	})
	if err != nil {
		msg := s.describe(err)
		if failed != "" {
			msg = "« " + failed + " » : " + msg
		}
		s.render(w, http.StatusBadRequest, "admin", s.adminPage(msg, input))
		return
	}
	redirect(w, r, "/admin")
}

func (s *Server) handleRemovePlayer(w http.ResponseWriter, r *http.Request) {
	id := game.PlayerID(r.PathValue("id"))
	s.adminUpdate(w, r, func(g *game.Game) error { return g.RemovePlayer(id) })
}

func (s *Server) handleDraw(w http.ResponseWriter, r *http.Request) {
	theme := s.catalog.Themes[r.PostFormValue("theme")]
	setup := s.catalog.Setups[r.PostFormValue("setup")]
	s.adminUpdate(w, r, func(g *game.Game) error {
		if theme == nil || setup == nil {
			return game.ErrInvalid
		}
		return g.Draw(theme, setup, s.opts.NewRand())
	})
}

func (s *Server) handleCancelDraw(w http.ResponseWriter, r *http.Request) {
	s.adminUpdate(w, r, func(g *game.Game) error { return g.CancelDraw() })
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	s.adminUpdate(w, r, func(g *game.Game) error { return g.Close() })
}

// handleReset starts a brand new game, forgetting players, roles and guesses.
// Player IDs keep increasing, so a player session from the old game never matches a new player.
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	s.adminUpdate(w, r, func(g *game.Game) error {
		lastID := g.LastID
		*g = *game.New()
		g.LastID = lastID
		return nil
	})
}
