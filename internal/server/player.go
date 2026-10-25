package server

import (
	"cmp"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/game"
)

const playerCookie = "spyball_player"

func (s *Server) registerPlayer(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", s.handleLoginForm)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /logout", s.handleLogout)

	mux.HandleFunc("GET /me", s.handleRole)
	mux.HandleFunc("POST /me/guesses", s.handleGuesses)
	mux.HandleFunc("POST /me/camp", s.handleCamp)
}

// The player cookie holds the player ID and character. It is only valid while that player still has
// that character, so a new draw or a new game logs everyone out.
func playerToken(p *game.Player) string {
	return string(p.ID) + "|" + config.NormalizeName(p.Character)
}

// currentPlayer returns the ID of the logged-in player, if the session is still valid.
func (s *Server) currentPlayer(r *http.Request) (game.PlayerID, bool) {
	token, ok := s.sessions.get(r, playerCookie)
	if !ok {
		return "", false
	}
	id, _, _ := strings.Cut(token, "|")
	valid := false
	_ = s.store.Read(func(g *game.Game) error {
		p, err := g.Player(game.PlayerID(id))
		valid = err == nil && g.Phase != game.PhaseSetup && playerToken(p) == token
		return nil
	})
	return game.PlayerID(id), valid
}

type loginPage struct {
	Error     string
	Phase     game.Phase
	Names     []string
	Name      string // kept on error
	NeedsChar bool   // the chosen player already logged in once
}

func (s *Server) loginPage(errMsg, name string, needsChar bool) loginPage {
	page := loginPage{Error: errMsg, Name: name, NeedsChar: needsChar}
	_ = s.store.Read(func(g *game.Game) error {
		page.Phase = g.Phase
		for _, p := range g.Players {
			page.Names = append(page.Names, p.Name)
		}
		return nil
	})
	slices.SortFunc(page.Names, func(a, b string) int { return cmp.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return page
}

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentPlayer(r); ok {
		redirect(w, r, "/me")
		return
	}
	s.render(w, http.StatusOK, "login", s.loginPage("", "", false))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	var token string
	var firstTime bool
	err := s.store.Update(func(g *game.Game) error {
		if p, ok := g.PlayerByName(name); ok {
			firstTime = !p.Revealed
		}
		p, err := g.Login(name, r.PostFormValue("character"))
		if err != nil {
			return err
		}
		token = playerToken(p)
		return nil
	})
	if err != nil {
		needsChar := errors.Is(err, game.ErrCharacterRequired) || errors.Is(err, game.ErrWrongCharacter)
		s.render(w, http.StatusBadRequest, "login", s.loginPage(s.describe(err), name, needsChar))
		return
	}

	s.sessions.set(w, playerCookie, token)
	if firstTime {
		redirect(w, r, "/me?welcome=1")
		return
	}
	redirect(w, r, "/me")
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.sessions.clear(w, playerCookie)
	redirect(w, r, "/")
}

type rolePage struct {
	Error   string
	Welcome bool // first login: insist on remembering the character
	Saved   bool // guesses just saved
	Closed  bool

	Name        string
	Character   string
	Description string
	Color       config.Color
	Rule        config.Rule
	TeamName    string
	TeamHelp    string

	Signs []roleSign
	Clues []game.Clue

	// White players only.
	Camps  []campOption
	Choice config.Camp

	Others []guessRow
}

type roleSign struct {
	For  string // team that recognizes this sign; empty when there is only one sign
	Text string
}

type campOption struct {
	Value config.Camp
	Name  string
}

type guessRow struct {
	ID    game.PlayerID
	Name  string
	Guess game.Guess
}

func (s *Server) rolePage(id game.PlayerID, errMsg string) (rolePage, error) {
	var page rolePage
	err := s.store.Read(func(g *game.Game) error {
		me, err := g.Player(id)
		if err != nil {
			return err
		}
		theme := s.catalog.Themes[g.ThemeID]
		if theme == nil {
			return errors.New("theme " + g.ThemeID + " is no longer available")
		}
		clues, err := g.Clues(id)
		if err != nil {
			return err
		}

		page = rolePage{
			Error:       errMsg,
			Closed:      g.Phase == game.PhaseClosed,
			Name:        me.Name,
			Character:   me.Character,
			Description: me.Description,
			Color:       me.Color,
			Rule:        me.Rule,
			Clues:       clues,
			Choice:      me.Camp(),
		}

		switch me.Color {
		case config.Blue, config.Red:
			team := theme.Teams[me.Camp()]
			page.TeamName, page.TeamHelp = team.Name, team.Description
		default:
			team := theme.Teams[config.CampNeutral]
			page.TeamName = team.Name
		}

		colors := me.Color.SignColors()
		for i, sign := range me.Signs {
			rs := roleSign{Text: sign}
			if len(colors) > 1 {
				rs.For = theme.Teams[config.Camp(colors[i])].Name
			}
			page.Signs = append(page.Signs, rs)
		}

		if me.Color == config.White {
			for _, camp := range config.Camps {
				page.Camps = append(page.Camps, campOption{Value: camp, Name: theme.Teams[camp].Name})
			}
		}

		for _, p := range g.Players {
			if p.ID != me.ID {
				page.Others = append(page.Others, guessRow{ID: p.ID, Name: p.Name, Guess: me.Guesses[p.ID]})
			}
		}
		slices.SortFunc(page.Others, func(a, b guessRow) int {
			return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
		return nil
	})
	return page, err
}

// withPlayer runs next for the logged-in player, or sends them to the login page.
func (s *Server) withPlayer(w http.ResponseWriter, r *http.Request, next func(id game.PlayerID)) {
	id, ok := s.currentPlayer(r)
	if !ok {
		s.sessions.clear(w, playerCookie)
		redirect(w, r, "/login")
		return
	}
	next(id)
}

func (s *Server) renderRole(w http.ResponseWriter, r *http.Request, status int, id game.PlayerID, errMsg string) {
	page, err := s.rolePage(id, errMsg)
	if err != nil {
		s.render(w, http.StatusInternalServerError, "error", errorPage{Message: s.describe(err)})
		return
	}
	if status == http.StatusOK {
		page.Welcome = r.URL.Query().Get("welcome") == "1"
		page.Saved = r.URL.Query().Get("saved") == "1"
	}
	s.render(w, status, "role", page)
}

func (s *Server) handleRole(w http.ResponseWriter, r *http.Request) {
	s.withPlayer(w, r, func(id game.PlayerID) {
		s.renderRole(w, r, http.StatusOK, id, "")
	})
}

// handleGuesses saves the whole guess form at once: one "guess_<player id>" field per other player.
func (s *Server) handleGuesses(w http.ResponseWriter, r *http.Request) {
	s.withPlayer(w, r, func(id game.PlayerID) {
		_ = r.ParseForm()
		err := s.store.Update(func(g *game.Game) error {
			if g.Phase != game.PhaseRunning {
				return game.ErrWrongPhase // even for a form without any guess
			}
			for _, p := range g.Players {
				values, ok := r.PostForm["guess_"+string(p.ID)]
				if !ok || p.ID == id {
					continue
				}
				if err := g.SetGuess(id, p.ID, game.Guess(values[0])); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			s.renderRole(w, r, http.StatusBadRequest, id, s.describe(err))
			return
		}
		redirect(w, r, "/me?saved=1#guesses")
	})
}

func (s *Server) handleCamp(w http.ResponseWriter, r *http.Request) {
	s.withPlayer(w, r, func(id game.PlayerID) {
		camp := config.Camp(r.PostFormValue("camp"))
		if err := s.store.Update(func(g *game.Game) error { return g.ChooseCamp(id, camp) }); err != nil {
			s.renderRole(w, r, http.StatusBadRequest, id, s.describe(err))
			return
		}
		redirect(w, r, "/me#camp")
	})
}

type errorPage struct {
	Message string
}
