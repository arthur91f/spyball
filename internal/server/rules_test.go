package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/arthur91f/spyball/internal/config"
)

func TestRulesPage(t *testing.T) {
	c := newTestClient(t)
	resp, body := c.get("/rules?theme=ball-of-nations")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Le bal des nations",
		"ONG P.L.S. (Pour La Santé)", // story
		"Démocraties du monde libre",
		"Puissances autoritaires",
		"Agent double",
		"un tous les 4 ou 5 joueurs",
		"2 points par allié que tu désignes comme allié",
		"-2 points par joueur qui te désigne comme ennemi, qu'il ait raison ou non",
		"4 points par défi réalisé, pour le camp des traîtres",
		"sans ses traîtres",
	} {
		if !strings.Contains(body, htmlEscape(want)) {
			t.Errorf("rules page lacks %q", want)
		}
	}
	if !strings.Contains(body, `<nav><a href="/rules">`) {
		t.Error("no rules link in the header")
	}
	if !strings.Contains(body, `<option value="halloween">La nuit des esprits</option>`) {
		t.Error("theme picker does not offer the other themes")
	}
}

func TestRulesShowNeitherCastNorSetup(t *testing.T) {
	admin := startedGame(t)
	_, body := admin.newPlayerClient().get("/rules")
	for _, s := range admin.srv.catalog.Setups {
		if strings.Contains(body, htmlEscape(s.Name)) {
			t.Errorf("rules page reveals the setup %q", s.Name)
		}
	}
	for _, p := range admin.game().Players {
		for _, secret := range []string{p.Name, p.Character} {
			if strings.Contains(body, htmlEscape(secret)) {
				t.Errorf("rules page shows %q", secret)
			}
		}
	}
}

func TestRulesThemeChoice(t *testing.T) {
	c := newTestClient(t)
	other := *c.srv.catalog.Themes["ball-of-nations"]
	other.ID, other.Name = "other", "Autre thème"
	other.Teams = map[config.Color]config.Team{
		config.Blue: {Name: "Lutins", Member: "lutin"},
		config.Red:  {Name: "Disciples du père Fouettard", Member: "disciple"},
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
