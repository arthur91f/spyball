package server

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/arthur91f/spyball/internal/game"
)

// closedGame plays a game where every player guesses about the next one, then closes it.
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
	if len(res.Teams) != 3 || !res.Teams[2].Traitors {
		t.Fatalf("teams = %+v, want blue, red and the traitors", res.Teams)
	}
	for _, team := range res.Teams {
		want := `<span class="badge team-` + string(team.Color) + `">`
		if team.Traitors {
			want = `<span class="badge team-traitors">Agents doubles</span>`
		}
		if !strings.Contains(body, want) || !strings.Contains(body, strconv.Itoa(team.Score)+"&nbsp;pts") {
			t.Errorf("team %+v not shown", team)
		}
	}
	if !strings.Contains(body, "2 traîtres, 0 défi(s) réalisé(s)") {
		t.Error("traitors' camp details not shown")
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

func TestChallengesDone(t *testing.T) {
	admin := startedGame(t)
	challenges := admin.game().Challenges
	if len(challenges) < 2 {
		t.Fatalf("%d challenges, want at least 2", len(challenges))
	}
	traitorsScore := func() int {
		teams := admin.game().Score().Teams
		return teams[len(teams)-1].Score
	}
	totals := func() []int {
		var totals []int
		for _, ps := range admin.game().Score().Players {
			totals = append(totals, ps.Total)
		}
		return totals
	}

	// Challenges can only be checked once the game is closed.
	if resp, _ := admin.post("/admin/challenges", url.Values{"done": {"0"}}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("check while running: status %d", resp.StatusCode)
	}
	admin.post("/admin/close", nil)
	before, totalsBefore := traitorsScore(), totals()

	// The admin page has one checkbox per challenge, and does not ask who did it.
	_, body := admin.get("/admin")
	for i, c := range challenges {
		if !strings.Contains(body, htmlEscape(c.Text)) || !strings.Contains(body, `<input type="checkbox" name="done" value="`+strconv.Itoa(i)+`">`) {
			t.Errorf("admin page lacks a checkbox for challenge %d %q", i, c.Text)
		}
	}
	if strings.Contains(body, `name="traitor"`) {
		t.Error("admin page asks which traitor did a challenge")
	}

	resp, body := admin.post("/admin/challenges", url.Values{"done": {"1"}})
	if resp.Request.URL.Path != "/admin" || !strings.Contains(body, `<input type="checkbox" name="done" value="1" checked>`) {
		t.Fatalf("check landed on %s:\n%s", resp.Request.URL, body)
	}
	// The points go to the traitors' camp, not to any player.
	if got := traitorsScore(); got != before+game.PointsChallenge {
		t.Errorf("traitors' camp: %d, want %d", got, before+game.PointsChallenge)
	}
	if got := totals(); !slices.Equal(got, totalsBefore) {
		t.Errorf("player totals changed: %v, want %v", got, totalsBefore)
	}

	// Everyone sees which challenges were done; only the admin gets a link to change it.
	visitor := admin.newPlayerClient()
	_, body = visitor.get("/results")
	for _, want := range []string{
		htmlEscape(challenges[0].Text) + ` »
            <small class="muted">non réalisé</small>`,
		htmlEscape(challenges[1].Text) + ` »
            <strong class="status validated">✓ réalisé</strong>`,
		"2 traîtres, 1 défi(s) réalisé(s)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("results page lacks %q", want)
		}
	}
	if strings.Contains(body, `href="/admin#challenges"`) {
		t.Error("visitor gets the link to check challenges")
	}
	if resp, _ := visitor.post("/admin/challenges", url.Values{"done": {"0"}}); resp.Request.URL.Path != "/admin/login" {
		t.Errorf("visitor checking landed on %s", resp.Request.URL.Path)
	}
	if _, body := admin.get("/results"); !strings.Contains(body, `href="/admin#challenges"`) {
		t.Error("admin gets no link to check challenges")
	}

	// Unchecking everything.
	admin.post("/admin/challenges", url.Values{})
	if got := traitorsScore(); got != before {
		t.Errorf("traitors' camp after unchecking = %d, want %d", got, before)
	}

	for _, bad := range []string{"99", "-1", "x"} {
		if resp, _ := admin.post("/admin/challenges", url.Values{"done": {bad}}); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("done=%s: status %d, want 400", bad, resp.StatusCode)
		}
	}
}
