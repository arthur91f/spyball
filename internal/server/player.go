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
	TeamName    string
	TeamHelp    string
	Sign        string

	Traitor    *config.Role // themed name of the traitors, for traitors only
	Challenges []string     // traitors only: the challenges shared by all the traitors
	Clues      []roleClue

	Points pointsHelp
	Others []guessRow
}

// roleClue is a clue about another player: a badge for their color, then their sign.
type roleClue struct {
	Color config.Color
	Label string
	Sign  string
}

// pointsHelp is the scoring rules, to explain them on the pages.
type pointsHelp struct {
	AllyFound, EnemyFound, Wrong, DesignatedAlly, DesignatedEnemy, Challenge int
}

var points = pointsHelp{
	AllyFound:       game.PointsAllyFound,
	EnemyFound:      game.PointsEnemyFound,
	Wrong:           game.PointsWrongGuess,
	DesignatedAlly:  game.PointsDesignatedAlly,
	DesignatedEnemy: game.PointsDesignatedEnemy,
	Challenge:       game.PointsChallenge,
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

		team := theme.Teams[me.Color]
		page = rolePage{
			Error:       errMsg,
			Closed:      g.Phase == game.PhaseClosed,
			Name:        me.Name,
			Character:   me.Character,
			Description: me.Description,
			Color:       me.Color,
			TeamName:    team.Name,
			TeamHelp:    team.Description,
			Sign:        me.Sign,
			Points:      points,
		}
		if me.Traitor {
			page.Traitor = &theme.Traitor
		}

		for _, c := range clues {
			page.Clues = append(page.Clues, roleClue{Color: c.Color, Label: theme.Teams[c.Color].Member, Sign: c.Sign})
		}
		if me.Traitor {
			for _, c := range g.Challenges {
				page.Challenges = append(page.Challenges, c.Text)
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

type errorPage struct {
	Message string
}
