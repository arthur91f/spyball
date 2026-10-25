package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/arthur91f/spyball/internal/game"
)

// testClient drives a test server like a browser: it keeps cookies and follows redirects.
type testClient struct {
	t      *testing.T
	srv    *Server
	ts     *httptest.Server
	client *http.Client
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &testClient{t: t, srv: srv, ts: ts, client: &http.Client{Jar: jar}}
}

func (c *testClient) do(req *http.Request) (*http.Response, string) {
	c.t.Helper()
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func (c *testClient) get(path string) (*http.Response, string) {
	c.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, c.ts.URL+path, nil)
	return c.do(req)
}

func (c *testClient) post(path string, form url.Values) (*http.Response, string) {
	c.t.Helper()
	req, _ := http.NewRequest(http.MethodPost, c.ts.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req)
}

func (c *testClient) game() *game.Game {
	var snapshot *game.Game
	_ = c.srv.store.Read(func(g *game.Game) error {
		copied := *g
		snapshot = &copied
		return nil
	})
	return snapshot
}

func (c *testClient) loginAdmin() {
	c.t.Helper()
	resp, _ := c.post("/admin/login", url.Values{"password": {testAdminPassword}})
	if resp.Request.URL.Path != "/admin" {
		c.t.Fatalf("admin login landed on %s, want /admin", resp.Request.URL.Path)
	}
}

func guestNames(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "Invité %d\n", i+1)
	}
	return b.String()
}

func TestAdminRequiresLogin(t *testing.T) {
	c := newTestClient(t)

	for _, path := range []string{"/admin"} {
		resp, _ := c.get(path)
		if resp.Request.URL.Path != "/admin/login" {
			t.Errorf("GET %s landed on %s, want /admin/login", path, resp.Request.URL.Path)
		}
	}
	resp, _ := c.post("/admin/reset", nil)
	if resp.Request.URL.Path != "/admin/login" {
		t.Errorf("POST /admin/reset landed on %s, want /admin/login", resp.Request.URL.Path)
	}

	resp, body := c.post("/admin/login", url.Values{"password": {"wrong"}})
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(body, "Mot de passe incorrect") {
		t.Errorf("wrong password: status %d, body %q", resp.StatusCode, body)
	}

	c.loginAdmin()
	resp, _ = c.post("/admin/logout", nil)
	resp, _ = c.get("/admin")
	if resp.Request.URL.Path != "/admin/login" {
		t.Errorf("after logout, /admin landed on %s", resp.Request.URL.Path)
	}
}

func TestAdminForgedCookieIsRejected(t *testing.T) {
	c := newTestClient(t)
	req, _ := http.NewRequest(http.MethodGet, c.ts.URL+"/admin", nil)
	req.AddCookie(&http.Cookie{Name: adminCookie, Value: "YWRtaW4.forged"})
	resp, _ := c.do(req)
	if resp.Request.URL.Path != "/admin/login" {
		t.Errorf("forged cookie reached %s", resp.Request.URL.Path)
	}
}

func TestAdminRejectsCrossSiteForms(t *testing.T) {
	c := newTestClient(t)
	c.loginAdmin()
	req, _ := http.NewRequest(http.MethodPost, c.ts.URL+"/admin/reset", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, _ := c.do(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site POST: status %d, want 403", resp.StatusCode)
	}
}

func TestAdminGameLifecycle(t *testing.T) {
	c := newTestClient(t)
	c.loginAdmin()

	// Players are added in bulk; a duplicate makes the whole batch fail and keeps the input.
	resp, body := c.post("/admin/players", url.Values{"names": {guestNames(11) + "invité 3\n"}})
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, "« invité 3 » : Ce nom est déjà utilisé.") {
		t.Fatalf("duplicate name: status %d, body:\n%s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "Invité 11") {
		t.Error("names input not kept after error")
	}
	if n := len(c.game().Players); n != 0 {
		t.Fatalf("%d players added by a failed batch, want 0", n)
	}

	_, body = c.post("/admin/players", url.Values{"names": {guestNames(11)}})
	if !strings.Contains(body, "Joueurs (11)") || !strings.Contains(body, "Aucune configuration n&#39;existe pour 11 joueurs") {
		t.Errorf("11 players: page does not say no setup fits:\n%s", body)
	}

	_, body = c.post("/admin/players", url.Values{"names": {"Douzième\n\n"}})
	if !strings.Contains(body, "Joueurs (12)") || !strings.Contains(body, `value="12-classic"`) {
		t.Errorf("12 players: classic setup not offered:\n%s", body)
	}

	// Remove then re-add one player.
	removed := c.game().Players[11]
	c.post("/admin/players/"+string(removed.ID)+"/delete", nil)
	if n := len(c.game().Players); n != 11 {
		t.Fatalf("after delete: %d players, want 11", n)
	}
	c.post("/admin/players", url.Values{"names": {"Douzième"}})

	resp, body = c.post("/admin/draw", url.Values{"theme": {"ball-of-nations"}, "setup": {"12-classic"}})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "Partie en cours") {
		t.Fatalf("draw: status %d, body:\n%s", resp.StatusCode, body)
	}
	g := c.game()
	if g.Phase != game.PhaseRunning {
		t.Fatalf("phase = %s, want running", g.Phase)
	}

	// The admin may play: the admin page must never reveal roles or signs.
	for _, p := range g.Players {
		if !strings.Contains(body, p.Name) {
			t.Errorf("player %s not listed", p.Name)
		}
		for _, secret := range append([]string{p.Character}, p.Signs...) {
			if strings.Contains(body, secret) || strings.Contains(body, htmlEscape(secret)) {
				t.Errorf("admin page reveals %q", secret)
			}
		}
	}

	// Players cannot be added once roles are drawn.
	resp, body = c.post("/admin/players", url.Values{"names": {"Retardataire"}})
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, "pas possible à ce stade") {
		t.Errorf("add after draw: status %d", resp.StatusCode)
	}

	_, body = c.post("/admin/close", nil)
	if c.game().Phase != game.PhaseClosed || !strings.Contains(body, "Partie terminée") {
		t.Fatalf("close: phase %s", c.game().Phase)
	}

	resp, _ = c.post("/admin/cancel-draw", nil)
	if resp.StatusCode != http.StatusBadRequest || c.game().Phase != game.PhaseClosed {
		t.Errorf("cancel after close: status %d, phase %s", resp.StatusCode, c.game().Phase)
	}

	c.post("/admin/reset", nil)
	if g := c.game(); g.Phase != game.PhaseSetup || len(g.Players) != 0 {
		t.Errorf("after reset: phase %s, %d players", g.Phase, len(g.Players))
	}
}

func TestAdminCancelDraw(t *testing.T) {
	c := newTestClient(t)
	c.loginAdmin()
	c.post("/admin/players", url.Values{"names": {guestNames(12)}})
	c.post("/admin/draw", url.Values{"theme": {"ball-of-nations"}, "setup": {"12-classic"}})

	_, body := c.post("/admin/cancel-draw", nil)
	if g := c.game(); g.Phase != game.PhaseSetup || len(g.Players) != 12 {
		t.Errorf("after cancel: phase %s, %d players", g.Phase, len(g.Players))
	}
	if !strings.Contains(body, "Tirage des rôles") {
		t.Error("setup page not shown after cancel")
	}
}

func TestAdminDrawRejectsUnknownSetup(t *testing.T) {
	c := newTestClient(t)
	c.loginAdmin()
	c.post("/admin/players", url.Values{"names": {guestNames(12)}})

	resp, body := c.post("/admin/draw", url.Values{"theme": {"ball-of-nations"}, "setup": {"nope"}})
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Requête invalide") {
		t.Errorf("unknown setup: status %d", resp.StatusCode)
	}
	if c.game().Phase != game.PhaseSetup {
		t.Error("game left setup phase")
	}
}

func htmlEscape(s string) string {
	return strings.NewReplacer("'", "&#39;", "&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;").Replace(s)
}
