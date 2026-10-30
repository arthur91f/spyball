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
		"-5 points par joueur qui te désigne comme ennemi, qu'il ait raison ou non",
		"10 points par défi réalisé, pour le camp des traîtres",
		"sans ses traîtres",
	} {
		if !strings.Contains(body, htmlEscape(want)) {
			t.Errorf("rules page lacks %q", want)
		}
	}
	if !strings.Contains(body, `<nav><a href="/rules">`) {
		t.Error("no rules link in the header")
	}
	if !strings.Contains(body, `<option value="halloween">La nuit des Esprits</option>`) {
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

func TestRulesThemeTexts(t *testing.T) {
	c := newTestClient(t)
	other := *c.srv.catalog.Themes["ball-of-nations"]
	other.ID, other.Name = "other", "Autre thème"
	other.Texts = map[string]string{
		"rules.camps.balance":   "Les **Esprits** cherchent leurs alliés <ici>.",
		"rules.role.character":  "Ton personnage\nGarde-le secret.",
		"rules.score.wrong":     "%d points par erreur d'identification ;",
		"rules.score.relations": "",
	}
	c.srv.catalog.Themes["other"] = &other
	const balance = "Les deux camps ne comptent pas forcément autant de joueurs."

	_, body := c.get("/rules?theme=other")
	for _, want := range []string{
		"Les <strong>Esprits</strong> cherchent leurs alliés &lt;ici&gt;.", // bold, and still escaped
		"Ton personnage<br>Garde-le secret.",
		"-2 points par erreur d&#39;identification ;",
		htmlEscape("Pour les traîtres, des défis"), // texts the theme does not replace come from the locale
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rules page lacks %q", want)
		}
	}
	if strings.Contains(body, htmlEscape(balance)) {
		t.Error("the theme's text did not replace the locale's")
	}
	if strings.Contains(body, htmlEscape("Deux joueurs du même camp sont alliés")) {
		t.Error("an empty theme text did not hide its paragraph")
	}

	// The other themes keep the locale's texts.
	_, body = c.get("/rules?theme=ball-of-nations")
	if !strings.Contains(body, htmlEscape(balance)) {
		t.Error("another theme lost the locale's text")
	}
}
