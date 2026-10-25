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
)

// TestFullGame plays a whole party through HTTP only, like the guests' phones would, and checks the
// results against an independent computation of the scores. It then restarts the server on the same
// data and checks that nothing was lost.
func TestFullGame(t *testing.T) {
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
	_, body := admin.post("/admin/draw", url.Values{"theme": {"ball-of-nations"}, "setup": {"12-classic"}})
	if !strings.Contains(body, "Partie en cours") {
		t.Fatalf("draw failed:\n%s", body)
	}

	// Every guest discovers their role on their own phone.
	type guest struct {
		name      string
		character string
		client    *testClient
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
		for _, m := range guessRe.FindAllStringSubmatch(body, -1) {
			g.guessIDs[m[2]] = html.UnescapeString(m[1])
		}
		if len(g.guessIDs) != len(guests)-1 {
			t.Fatalf("%s can guess about %d players, want %d", name, len(g.guessIDs), len(guests)-1)
		}
		byName[name] = g
		all = append(all, g)
	}

	// The admin page never leaks roles, even with everyone logged in.
	_, body = admin.get("/admin")
	for _, g := range all {
		if strings.Contains(body, htmlEscape(g.character)) {
			t.Errorf("admin page reveals %s", g.character)
		}
	}

	// Where each character belongs, from the theme: the oracle the results are checked against.
	colorOf, ruleOf := map[string]config.Color{}, map[string]config.Rule{}
	for color, chars := range theme.Characters {
		for _, c := range chars {
			colorOf[c.Name], ruleOf[c.Name] = color, c.Rule
		}
	}
	campOf := map[string]config.Camp{}
	whiteChoices := []config.Camp{config.CampRed, config.CampNeutral}
	for _, g := range all {
		switch colorOf[g.character] {
		case config.Blue:
			campOf[g.name] = config.CampBlue
		case config.Red:
			campOf[g.name] = config.CampRed
		case config.White:
			campOf[g.name] = whiteChoices[0]
			whiteChoices = whiteChoices[1:]
			g.client.post("/me/camp", url.Values{"camp": {string(campOf[g.name])}})
		default:
			campOf[g.name] = config.CampNeutral
		}
	}
	isAlly := func(a, b *guest) bool {
		ra, rb := ruleOf[a.character], ruleOf[b.character]
		switch {
		case ra == config.AllyOfEveryone || rb == config.AllyOfEveryone:
			return true
		case ra == config.EnemyOfEveryone || rb == config.EnemyOfEveryone:
			return false
		}
		return campOf[a.name] == campOf[b.name]
	}

	// Everyone guesses about everyone: right for the first five, wrong for the next three, no answer otherwise.
	want := map[string]int{}
	for _, g := range all {
		form := url.Values{}
		n := 0
		for id, otherName := range g.guessIDs {
			other := byName[otherName]
			truth := "enemy"
			if isAlly(g, other) {
				truth = "ally"
			}
			opposite := map[string]string{"ally": "enemy", "enemy": "ally"}[truth]
			switch {
			case n < 5:
				form.Set("guess_"+id, truth)
				want[g.name]++
			case n < 8:
				form.Set("guess_"+id, opposite)
				want[g.name]--
			default:
				form.Set("guess_"+id, "")
			}
			n++
		}
		if resp, _ := g.client.post("/me/guesses", form); resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: saving guesses: status %d", g.name, resp.StatusCode)
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

	// A successful challenge, then the results.
	var challenger *guest
	for _, g := range all {
		if colorOf[g.character] == config.Yellow {
			challenger = g
			break
		}
	}
	_, resultsPage := admin.get("/results")
	optionRe := regexp.MustCompile(`<option value="(p\d+)">` + regexp.QuoteMeta(htmlEscape(challenger.name)) + `</option>`)
	m := optionRe.FindStringSubmatch(resultsPage)
	if m == nil {
		t.Fatalf("no option for %s in the points form", challenger.name)
	}
	admin.post("/results/points", url.Values{"player": {m[1]}, "points": {"2"}, "reason": {"Défi réussi"}})
	want[challenger.name] += 2

	visitor := admin.newPlayerClient()
	_, resultsPage = visitor.get("/results")
	rowRe := regexp.MustCompile(`<strong>([^<]+)</strong> · ([^<]+)<br>[\s\S]*?<span class="score">(-?\d+)</span>`)
	rows := rowRe.FindAllStringSubmatch(resultsPage, -1)
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
