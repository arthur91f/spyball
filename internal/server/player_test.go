package server

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/game"
)

// newPlayerClient returns a second browser on the same server, with its own cookies.
func (c *testClient) newPlayerClient() *testClient {
	jar, _ := cookiejar.New(nil)
	return &testClient{t: c.t, srv: c.srv, ts: c.ts, client: &http.Client{Jar: jar}}
}

// startedGame returns an admin client on a server whose 12-player game has been drawn.
func startedGame(t *testing.T) *testClient {
	t.Helper()
	admin := newTestClient(t)
	admin.loginAdmin()
	admin.post("/admin/players", url.Values{"names": {guestNames(12)}})
	admin.post("/admin/draw", url.Values{"theme": {"ball-of-nations"}, "setup": {"12-classic"}})
	if admin.game().Phase != game.PhaseRunning {
		t.Fatal("game not started")
	}
	return admin
}

func (c *testClient) playerBy(match func(p *game.Player) bool) *game.Player {
	c.t.Helper()
	for _, p := range c.game().Players {
		if match(p) {
			return p
		}
	}
	c.t.Fatal("no matching player")
	return nil
}

func (c *testClient) loginPlayer(name, character string) (*http.Response, string) {
	c.t.Helper()
	return c.post("/login", url.Values{"name": {name}, "character": {character}})
}

func TestLoginBeforeDraw(t *testing.T) {
	c := newTestClient(t)
	_, body := c.get("/login")
	if !strings.Contains(body, "pas encore commencé") {
		t.Errorf("login page before draw:\n%s", body)
	}
	if resp, _ := c.get("/me"); resp.Request.URL.Path != "/login" {
		t.Errorf("/me without session landed on %s", resp.Request.URL.Path)
	}
}

func TestPlayerLoginAndRolePage(t *testing.T) {
	admin := startedGame(t)
	me := admin.playerBy(func(p *game.Player) bool { return p.Color == config.Blue && len(p.Clues) > 0 })
	c := admin.newPlayerClient()

	_, body := c.get("/login")
	if !strings.Contains(body, `<option value="`+me.Name+`">`) {
		t.Fatalf("login page does not list %s", me.Name)
	}

	// First login: name only, the character is revealed with a warning to remember it.
	resp, body := c.loginPlayer(me.Name, "")
	if resp.Request.URL.Path != "/me" || !strings.Contains(body, "Retiens bien le nom de ton personnage") {
		t.Fatalf("first login landed on %s:\n%s", resp.Request.URL, body)
	}
	if !strings.Contains(body, `<h2 class="character">`+htmlEscape(me.Character)+`</h2>`) {
		t.Errorf("role page does not show the character %q", me.Character)
	}
	if !strings.Contains(body, htmlEscape(me.Signs[0])) {
		t.Errorf("role page does not show the sign %q", me.Signs[0])
	}

	// Clues show the clue characters and the signs this player may see, nothing about anyone else.
	g := admin.game()
	for _, p := range g.Players {
		if p.ID == me.ID {
			continue
		}
		isClue := slices.Contains(me.Clues, p.ID)
		shown := strings.Contains(body, "<strong>"+htmlEscape(p.Character)+"</strong>")
		if shown != isClue {
			t.Errorf("character %s shown=%v, is a clue=%v", p.Character, shown, isClue)
		}
		for i, sign := range p.Signs {
			visible := isClue && (p.Color != config.White || p.Color.SignColors()[i] == config.Blue)
			if got := strings.Contains(body, htmlEscape(sign)); got != visible {
				t.Errorf("sign %q of %s (%s) shown=%v, want %v", sign, p.Character, p.Color, got, visible)
			}
		}
	}
	if strings.Contains(body, `id="camp"`) {
		t.Error("camp choice shown to a blue player")
	}

	// Logging in again needs the character.
	c.post("/logout", nil)
	if resp, _ := c.get("/me"); resp.Request.URL.Path != "/login" {
		t.Fatal("still logged in after logout")
	}
	resp, body = c.loginPlayer(me.Name, "")
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, "indique aussi le nom de ton personnage") {
		t.Errorf("second login without character: status %d", resp.StatusCode)
	}
	if !strings.Contains(body, `<option value="`+me.Name+`" selected>`) {
		t.Error("chosen name not kept after error")
	}
	resp, _ = c.loginPlayer(me.Name, "Atlantide")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("wrong character: status %d", resp.StatusCode)
	}
	resp, body = c.loginPlayer(me.Name, strings.ToLower(me.Character))
	if resp.Request.URL.Path != "/me" || strings.Contains(body, "Retiens bien") {
		t.Errorf("login with character landed on %s", resp.Request.URL)
	}
}

func TestPlayerGuesses(t *testing.T) {
	admin := startedGame(t)
	g := admin.game()
	me, ally, enemy := g.Players[0], g.Players[1], g.Players[2]
	c := admin.newPlayerClient()
	c.loginPlayer(me.Name, "")

	resp, body := c.post("/me/guesses", url.Values{
		"guess_" + string(ally.ID):  {"ally"},
		"guess_" + string(enemy.ID): {"enemy"},
		"guess_" + string(me.ID):    {"ally"}, // ignored: no guess about yourself
	})
	if resp.Request.URL.Path != "/me" || !strings.Contains(body, "Suppositions enregistrées") {
		t.Fatalf("save guesses landed on %s", resp.Request.URL)
	}
	guesses := admin.playerBy(func(p *game.Player) bool { return p.ID == me.ID }).Guesses
	if len(guesses) != 2 || guesses[ally.ID] != game.GuessAlly || guesses[enemy.ID] != game.GuessEnemy {
		t.Errorf("guesses = %v", guesses)
	}
	if !strings.Contains(body, `id="g-`+string(ally.ID)+`-ally" name="guess_`+string(ally.ID)+`" value="ally" class="ally" checked>`) {
		t.Error("saved guess not checked on the page")
	}

	c.post("/me/guesses", url.Values{"guess_" + string(ally.ID): {""}})
	guesses = admin.playerBy(func(p *game.Player) bool { return p.ID == me.ID }).Guesses
	if len(guesses) != 1 {
		t.Errorf("after clearing one guess: %v", guesses)
	}

	resp, _ = c.post("/me/guesses", url.Values{"guess_" + string(ally.ID): {"maybe"}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid guess value: status %d", resp.StatusCode)
	}

	// Once the game is closed, the page is read-only.
	admin.post("/admin/close", nil)
	_, body = c.get("/me")
	if !strings.Contains(body, "La partie est terminée") || !strings.Contains(body, `<fieldset class="plain" disabled>`) {
		t.Error("closed game: role page not read-only")
	}
	resp, _ = c.post("/me/guesses", url.Values{"guess_" + string(ally.ID): {"ally"}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("guess after close: status %d", resp.StatusCode)
	}
}

func TestWhiteChoosesCamp(t *testing.T) {
	admin := startedGame(t)
	white := admin.playerBy(func(p *game.Player) bool { return p.Color == config.White })
	blue := admin.playerBy(func(p *game.Player) bool { return p.Color == config.Blue })

	c := admin.newPlayerClient()
	_, body := c.loginPlayer(white.Name, "")
	if !strings.Contains(body, `id="camp"`) || !strings.Contains(body, "Pour te faire reconnaître des Démocraties du monde libre") {
		t.Fatalf("white role page lacks camp choice or labelled signs:\n%s", body)
	}

	_, body = c.post("/me/camp", url.Values{"camp": {"red"}})
	if got := admin.playerBy(func(p *game.Player) bool { return p.ID == white.ID }).Choice; got != config.CampRed {
		t.Errorf("choice = %q, want red", got)
	}
	if !strings.Contains(body, `value="red" checked`) {
		t.Error("choice not checked on the page")
	}

	b := admin.newPlayerClient()
	b.loginPlayer(blue.Name, "")
	if resp, _ := b.post("/me/camp", url.Values{"camp": {"red"}}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("blue choosing a camp: status %d", resp.StatusCode)
	}
}

func TestPlayerSessionEndsWithTheDraw(t *testing.T) {
	admin := startedGame(t)
	p := admin.game().Players[0]
	c := admin.newPlayerClient()
	c.loginPlayer(p.Name, "")

	admin.post("/admin/cancel-draw", nil)
	if resp, _ := c.get("/me"); resp.Request.URL.Path != "/login" {
		t.Errorf("after cancel-draw, /me landed on %s", resp.Request.URL.Path)
	}
}

func TestAdminResetsForgottenCharacter(t *testing.T) {
	admin := startedGame(t)
	p := admin.game().Players[0]
	c := admin.newPlayerClient()
	c.loginPlayer(p.Name, "")

	_, body := admin.get("/admin")
	if !strings.Contains(body, "/admin/players/"+string(p.ID)+"/reset-login") {
		t.Fatal("no reset button for a logged-in player")
	}
	admin.post("/admin/players/"+string(p.ID)+"/reset-login", nil)

	other := admin.newPlayerClient()
	if resp, _ := other.loginPlayer(p.Name, ""); resp.Request.URL.Path != "/me" {
		t.Errorf("login with name only after reset landed on %s", resp.Request.URL.Path)
	}
}
