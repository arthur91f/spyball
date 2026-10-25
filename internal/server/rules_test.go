package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/arthur91f/spyball/internal/config"
)

func TestRulesPage(t *testing.T) {
	c := newTestClient(t)
	resp, body := c.get("/rules")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Le bal des nations",
		"ONG P.L.S. (Pour La Santé)", // story
		"Démocraties du monde libre",
		"Puissances autoritaires",
		"Non-alignés",
		"ONG P.L.S. : allié de tout le monde",
		"Anonymous : ennemi de tout le monde",
		"multiplié par 4 divisé par leur nombre",
	} {
		if !strings.Contains(body, htmlEscape(want)) {
			t.Errorf("rules page lacks %q", want)
		}
	}
	if !strings.Contains(body, `<nav><a href="/rules">`) {
		t.Error("no rules link in the header")
	}
	if strings.Contains(body, "Les personnages de cette partie") {
		t.Error("cast shown before the draw")
	}
	if strings.Contains(body, `name="theme"`) {
		t.Error("theme picker shown with a single theme")
	}
}

func TestRulesListCharactersInPlayButNotPlayers(t *testing.T) {
	admin := startedGame(t)
	_, body := admin.newPlayerClient().get("/rules")
	if !strings.Contains(body, "Les personnages de cette partie") {
		t.Fatal("cast not shown after the draw")
	}
	for _, p := range admin.game().Players {
		if !strings.Contains(body, htmlEscape(p.Character)) {
			t.Errorf("character %s not listed", p.Character)
		}
		if strings.Contains(body, htmlEscape(p.Name)) {
			t.Errorf("rules page shows player name %s", p.Name)
		}
	}
}

func TestRulesThemeChoice(t *testing.T) {
	c := newTestClient(t)
	other := *c.srv.catalog.Themes["ball-of-nations"]
	other.ID, other.Name = "other", "Autre thème"
	other.Teams = map[config.Camp]config.Team{
		config.CampBlue:    {Name: "Lutins", Member: "lutin"},
		config.CampRed:     {Name: "Disciples du père Fouettard", Member: "disciple"},
		config.CampNeutral: {Name: "Rennes", Member: "renne"},
	}
	c.srv.catalog.Themes["other"] = &other

	_, body := c.get("/rules?theme=other")
	if !strings.Contains(body, "Autre thème") || !strings.Contains(body, "Lutins") || !strings.Contains(body, `<option value="other" selected>`) {
		t.Errorf("theme=other not rendered:\n%s", body)
	}
	_, body = c.get("/rules?theme=nope")
	if !strings.Contains(body, "Le bal des nations") && !strings.Contains(body, "Autre thème") {
		t.Error("unknown theme did not fall back to a known one")
	}
}
