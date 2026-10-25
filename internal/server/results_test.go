package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/game"
)

// closedGame plays a game where every player guesses about the next one, a white joins red, then closes it.
func closedGame(t *testing.T) *testClient {
	t.Helper()
	admin := startedGame(t)
	err := admin.srv.store.Update(func(g *game.Game) error {
		for i, p := range g.Players {
			next := g.Players[(i+1)%len(g.Players)]
			guess := game.GuessAlly
			if i%3 == 0 {
				guess = game.GuessEnemy
			}
			if err := g.SetGuess(p.ID, next.ID, guess); err != nil {
				return err
			}
			if p.Color == config.White {
				_ = g.ChooseCamp(p.ID, config.CampRed)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	admin.post("/admin/close", nil)
	return admin
}

func TestResultsHiddenUntilClosed(t *testing.T) {
	admin := startedGame(t)
	visitor := admin.newPlayerClient()
	_, body := visitor.get("/results")
	if !strings.Contains(body, "Les résultats seront visibles") {
		t.Errorf("results before close:\n%s", body)
	}
	for _, p := range admin.game().Players {
		if strings.Contains(body, htmlEscape(p.Character)) {
			t.Errorf("results page reveals %s before close", p.Character)
		}
	}
	if _, body := visitor.get("/"); strings.Contains(body, `href="/results"`) {
		t.Error("home links to results before close")
	}
}

func TestResultsPage(t *testing.T) {
	admin := closedGame(t)
	visitor := admin.newPlayerClient()
	resp, body := visitor.get("/results")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	g := admin.game()
	res := g.Score()
	for _, p := range g.Players {
		if !strings.Contains(body, "<strong>"+htmlEscape(p.Name)+"</strong> · "+htmlEscape(p.Character)) {
			t.Errorf("player %s / %s not revealed", p.Name, p.Character)
		}
	}
	for _, team := range res.Teams {
		want := admin.srv.formatScore(team.Score) + "&nbsp;pts"
		if !strings.Contains(body, want) {
			t.Errorf("team %s score %q not shown", team.Camp, want)
		}
	}
	if !strings.Contains(body, "Victoire !") {
		t.Error("no winner shown")
	}
	if !strings.Contains(body, "✓") || !strings.Contains(body, "✗") {
		t.Error("guess details not shown")
	}
	if strings.Contains(body, `action="/results/points"`) {
		t.Error("visitor sees the admin points form")
	}
	if _, body := visitor.get("/"); !strings.Contains(body, `href="/results"`) {
		t.Error("home does not link to results after close")
	}

	_, body = admin.get("/results")
	if !strings.Contains(body, `action="/results/points"`) {
		t.Error("admin does not see the points form")
	}
}

func TestFormatScore(t *testing.T) {
	srv := newTestServer(t)
	for score, want := range map[float64]string{4: "4", -2: "-2", 16.0 / 3: "5,3", 0: "0"} {
		if got := srv.formatScore(score); got != want {
			t.Errorf("formatScore(%v) = %q, want %q", score, got, want)
		}
	}
}

func TestManualPoints(t *testing.T) {
	admin := closedGame(t)
	p := admin.game().Players[0]
	totalOf := func() int {
		for _, ps := range admin.game().Score().Players {
			if ps.Player == p.ID {
				return ps.Total
			}
		}
		return 0
	}
	before := totalOf()

	visitor := admin.newPlayerClient()
	if resp, _ := visitor.post("/results/points", url.Values{"player": {string(p.ID)}, "points": {"5"}}); resp.Request.URL.Path != "/admin/login" {
		t.Errorf("visitor adding points landed on %s", resp.Request.URL.Path)
	}

	resp, body := admin.post("/results/points", url.Values{"player": {string(p.ID)}, "points": {"+2"}, "reason": {"Défi réussi"}})
	if resp.Request.URL.Path != "/results" || !strings.Contains(body, "<strong>&#43;2</strong> "+htmlEscape(p.Name)) || !strings.Contains(body, "Défi réussi") {
		t.Fatalf("add points landed on %s:\n%s", resp.Request.URL, body)
	}
	admin.post("/results/points", url.Values{"player": {string(p.ID)}, "points": {"-1"}, "reason": {"Dénoncé publiquement"}})
	if got := totalOf(); got != before+1 {
		t.Errorf("total = %d, want %d", got, before+1)
	}

	for _, bad := range []string{"abc", "0", "50", ""} {
		resp, _ := admin.post("/results/points", url.Values{"player": {string(p.ID)}, "points": {bad}})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("points %q: status %d, want 400", bad, resp.StatusCode)
		}
	}
	if resp, _ := admin.post("/results/points", url.Values{"player": {"nobody"}, "points": {"1"}}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown player: status %d", resp.StatusCode)
	}

	admin.post("/results/points/0/delete", nil)
	if got := totalOf(); got != before-1 {
		t.Errorf("after removing +2: total = %d, want %d", got, before-1)
	}
	if n := len(admin.game().ManualPoints); n != 1 {
		t.Errorf("%d manual entries left, want 1", n)
	}
	if resp, _ := admin.post("/results/points/9/delete", nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("remove unknown entry: status %d", resp.StatusCode)
	}
}

func TestManualPointsOnlyAfterClose(t *testing.T) {
	admin := startedGame(t)
	p := admin.game().Players[0]
	resp, _ := admin.post("/results/points", url.Values{"player": {string(p.ID)}, "points": {"1"}})
	if resp.StatusCode != http.StatusBadRequest || len(admin.game().ManualPoints) != 0 {
		t.Errorf("points before close: status %d", resp.StatusCode)
	}
}
