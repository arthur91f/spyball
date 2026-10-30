package server

import (
	"html"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/game"
)

// TestFullGame plays a whole party through HTTP only, like the guests' phones would, and checks the
// results against an independent computation of the scores. It then restarts the server on the same
// data and checks that nothing was lost.
func TestFullGame(t *testing.T) {
	for _, setup := range []string{"12-classic", "12-traitors"} {
		t.Run(setup, func(t *testing.T) { playFullGame(t, setup) })
	}
}

func playFullGame(t *testing.T, setupID string) {
	dataDir := t.TempDir()
	srv := newTestServerAt(t, dataDir)
	adminTS := httptest.NewServer(srv)
	t.Cleanup(adminTS.Close)
	adminJar, _ := cookiejar.New(nil)
	admin := &testClient{t: t, srv: srv, ts: adminTS, client: &http.Client{Jar: adminJar}}
	theme := admin.srv.catalog.Themes["ball-of-nations"]

	// The organizer sets the game up.
	admin.loginAdmin()
	guests := []string{"Rémi", "Zoé", "Anaïs", "Benoît", "Chloé", "Damien", "Élodie", "François", "Gaëlle", "Hugo", "Inès", "Jérôme"}
	admin.post("/admin/players", url.Values{"names": {strings.Join(guests, "\n")}})
	// The setup is drawn at random by weight: only the one under test can be drawn.
	for id, s := range srv.catalog.Setups {
		if id != setupID {
			s.Weight = 0
		}
	}
	_, body := admin.post("/admin/draw", url.Values{"theme": {"ball-of-nations"}})
	if !strings.Contains(body, "Partie en cours") {
		t.Fatalf("draw failed:\n%s", body)
	}

	// Every guest discovers their role on their own phone.
	type guest struct {
		name      string
		character string
		client    *testClient
		rolePage  string
		guessIDs  map[string]string // guess field player ID -> guest name
	}
	byName := map[string]*guest{}
	var all []*guest
	characterRe := regexp.MustCompile(`<h2 class="character">([^<]+)</h2>`)
	guessRe := regexp.MustCompile(`<span class="guess-name">([^<]+)</span>\s*<span class="choice" role="radiogroup" aria-label="[^"]*">\s*<input type="radio" id="g-(p\d+)-ally"`)

	_, loginPage := admin.newPlayerClient().get("/login")
	for _, name := range guests {
		if !strings.Contains(loginPage, `<option value="`+name+`">`) {
			t.Fatalf("login page does not offer %s", name)
		}
		g := &guest{name: name, client: admin.newPlayerClient(), guessIDs: map[string]string{}}
		_, body := g.client.loginPlayer(name, "")
		m := characterRe.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("%s: no character on the role page:\n%s", name, body)
		}
		g.character = html.UnescapeString(m[1])
		g.rolePage = body
		for _, m := range guessRe.FindAllStringSubmatch(body, -1) {
			g.guessIDs[m[2]] = html.UnescapeString(m[1])
		}
		if len(g.guessIDs) != len(guests)-1 {
			t.Fatalf("%s can guess about %d players, want %d", name, len(g.guessIDs), len(guests)-1)
		}
		byName[name] = g
		all = append(all, g)
	}

	// A role page shows no other character, not even in the clues: characters are passwords.
	for _, g := range all {
		for _, other := range all {
			if other != g && strings.Contains(g.rolePage, htmlEscape(other.character)) {
				t.Errorf("role page of %s reveals %s", g.name, other.character)
			}
		}
	}

	// The admin page never leaks roles, even with everyone logged in.
	_, body = admin.get("/admin")
	for _, g := range all {
		if strings.Contains(body, htmlEscape(g.character)) {
			t.Errorf("admin page reveals %s", g.character)
		}
	}

	// Where each character belongs, from the theme, and who the traitors are, from their role page:
	// the oracle the results are checked against.
	colorOf := map[string]config.Color{}
	for color, chars := range theme.Characters {
		for _, c := range chars {
			colorOf[c.Name] = color
		}
	}
	isTraitor := map[string]bool{}
	traitorsByColor := map[config.Color]int{}
	for _, g := range all {
		if strings.Contains(g.rolePage, `<span class="badge traitor">`) {
			isTraitor[g.name] = true
			traitorsByColor[colorOf[g.character]]++
		}
	}
	setup := admin.srv.catalog.Setups[setupID]
	for _, color := range config.Colors {
		if traitorsByColor[color] != setup.CountTraitors(color) {
			t.Fatalf("%d %s traitors, want %d", traitorsByColor[color], color, setup.CountTraitors(color))
		}
	}

	// Everyone guesses about everyone: right for the first five, wrong for the next three, no answer otherwise.
	// A right guess earns 2 points. Right or wrong, the player it is about earns 1 if designated as an ally
	// and loses 2 if designated as an enemy.
	want := map[string]int{}
	for _, g := range all {
		form := url.Values{}
		n := 0
		for id, otherName := range g.guessIDs {
			other := byName[otherName]
			truth := "enemy"
			if colorOf[g.character] == colorOf[other.character] {
				truth = "ally"
			}
			opposite := map[string]string{"ally": "enemy", "enemy": "ally"}[truth]
			guess := ""
			switch {
			case n < 5:
				guess = truth
				want[g.name] += 2
			case n < 8:
				guess = opposite
				want[g.name] -= 2
			}
			switch guess {
			case "ally":
				want[other.name] += 3
			case "enemy":
				want[other.name] -= 5
			}
			form.Set("guess_"+id, guess)
			n++
		}
		if resp, _ := g.client.post("/me/guesses", form); resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: saving guesses: status %d", g.name, resp.StatusCode)
		}
	}

	// Every traitor sees the same challenges, and nobody else sees them. Traitors declare nothing.
	challengesRe := regexp.MustCompile(`(?s)<section class="card" id="challenges">.*?</section>`)
	var traitors []*guest
	for _, g := range all {
		challenges := challengesRe.FindString(g.rolePage)
		if !isTraitor[g.name] {
			if challenges != "" {
				t.Fatalf("%s is not a traitor but has challenges", g.name)
			}
			continue
		}
		traitors = append(traitors, g)
		if n := strings.Count(challenges, "<li>"); n != setup.Challenges {
			t.Fatalf("traitor %s has %d challenges, want %d", g.name, n, setup.Challenges)
		}
		if want := challengesRe.FindString(traitors[0].rolePage); challenges != want {
			t.Errorf("challenges of %s differ from those of %s:\n%s\n%s", g.name, traitors[0].name, challenges, want)
		}
		if strings.Contains(g.rolePage, `action="/me/challenges`) {
			t.Errorf("traitor %s can declare challenges", g.name)
		}
	}

	// A guest comes back later and types their character sloppily.
	remi := byName["Rémi"]
	remi.client.post("/logout", nil)
	sloppy := strings.ToLower(strings.NewReplacer(".", "", "-", " ", "é", "e", "É", "e", "ï", "i").Replace(remi.character))
	if resp, _ := remi.client.loginPlayer("Rémi", sloppy); resp.Request.URL.Path != "/me" {
		t.Errorf("Rémi could not log back in with %q (character %q)", sloppy, remi.character)
	}

	// The organizer closes the game: guesses are frozen.
	admin.post("/admin/close", nil)
	if resp, _ := remi.client.post("/me/guesses", url.Values{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("guesses accepted after close: status %d", resp.StatusCode)
	}

	// On the admin page, the organizer checks the challenges done (all but the first one), without saying by whom:
	// they score for the traitors' camp. Then they penalize a public denunciation.
	_, resultsPage := admin.get("/results")
	idOf := func(name string) string {
		m := regexp.MustCompile(`<option value="(p\d+)">` + regexp.QuoteMeta(htmlEscape(name)) + `</option>`).FindStringSubmatch(resultsPage)
		if m == nil {
			t.Fatalf("no option for %s in the points form", name)
		}
		return m[1]
	}
	_, adminPage := admin.get("/admin")
	if n := strings.Count(adminPage, `type="checkbox" name="done"`); n != setup.Challenges {
		t.Errorf("%d challenges to check, want %d", n, setup.Challenges)
	}
	done := url.Values{}
	for i := 1; i < setup.Challenges; i++ {
		done.Add("done", strconv.Itoa(i))
	}
	if resp, _ := admin.post("/admin/challenges", done); resp.StatusCode != http.StatusOK {
		t.Fatalf("checking the challenges done: status %d", resp.StatusCode)
	}
	challengesDone := setup.Challenges - 1

	admin.post("/results/points", url.Values{"player": {idOf("Zoé")}, "points": {"-1"}, "reason": {"Dénoncée publiquement"}})
	want["Zoé"]--

	visitor := admin.newPlayerClient()
	_, resultsPage = visitor.get("/results")
	if want := htmlEscape("Configuration tirée au sort : " + setup.Name); !strings.Contains(resultsPage, want) {
		t.Errorf("results page does not name the drawn setup %q", setup.Name)
	}
	section := func(title string) string {
		_, after, ok := strings.Cut(resultsPage, "<h2>"+title+"</h2>")
		if !ok {
			t.Fatalf("results page has no %q section", title)
		}
		before, _, _ := strings.Cut(after, "</section>")
		return before
	}

	// Every player, traitors included, with their character and score.
	rowRe := regexp.MustCompile(`<strong>([^<]+)</strong> · ([^<]+)<br>[\s\S]*?<span class="score">(-?\d+)</span>`)
	rows := rowRe.FindAllStringSubmatch(section("Classement des joueurs"), -1)
	if len(rows) != len(guests) {
		t.Fatalf("%d players in the results, want %d", len(rows), len(guests))
	}
	for _, row := range rows {
		name, character := html.UnescapeString(row[1]), html.UnescapeString(row[2])
		total, _ := strconv.Atoi(row[3])
		if character != byName[name].character {
			t.Errorf("results: %s played %s, want %s", name, character, byName[name].character)
		}
		if total != want[name] {
			t.Errorf("results: %s scored %d, want %d", name, total, want[name])
		}
	}

	// A color camp scores the sum of its members who are not traitors; the traitors' camp scores the sum of
	// the traitors, plus the challenges done.
	teams := section("Classement des camps")
	for _, color := range config.Colors {
		sum := 0
		for _, g := range all {
			if colorOf[g.character] == color && !isTraitor[g.name] {
				sum += want[g.name]
			}
		}
		wantTeam := `<span class="badge team-` + string(color) + `">` + htmlEscape(theme.Teams[color].Name) + `</span>
            <span class="score">` + strconv.Itoa(sum) + `&nbsp;pts</span>`
		if !strings.Contains(teams, wantTeam) {
			t.Errorf("team %s: score %d not shown in:\n%s", color, sum, teams)
		}
	}
	sum := challengesDone * game.PointsChallenge
	for _, g := range traitors {
		sum += want[g.name]
	}
	wantTeam := `<span class="badge team-traitors">` + htmlEscape(theme.Traitor.Team) + `</span>
            <span class="score">` + strconv.Itoa(sum) + `&nbsp;pts</span>
            <small class="muted">` + strconv.Itoa(len(traitors)) + ` traîtres, ` + strconv.Itoa(challengesDone) + ` défi(s) réalisé(s)</small>`
	if !strings.Contains(teams, wantTeam) {
		t.Errorf("traitors' camp: score %d not shown in:\n%s", sum, teams)
	}

	// The server restarts (power cut, laptop closed…): same results, sessions still valid.
	restarted := newTestServerAt(t, dataDir)
	ts := httptest.NewServer(restarted)
	t.Cleanup(ts.Close)
	again := &testClient{t: t, srv: restarted, ts: ts, client: &http.Client{}}
	again.client.Jar, _ = cookiejar.New(nil)
	_, afterRestart := again.get("/results")
	if afterRestart != resultsPage {
		t.Error("results changed after a restart")
	}

	oldURL, _ := url.Parse(remi.client.ts.URL)
	newURL, _ := url.Parse(ts.URL)
	again.client.Jar.SetCookies(newURL, remi.client.client.Jar.Cookies(oldURL))
	if resp, body := again.get("/me"); resp.Request.URL.Path != "/me" || !strings.Contains(body, htmlEscape(remi.character)) {
		t.Errorf("Rémi's session lost after restart: landed on %s", resp.Request.URL.Path)
	}
}
