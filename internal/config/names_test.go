package config

import "testing"

func TestNormalizeName(t *testing.T) {
	same := [][]string{
		{"ONG P.L.S.", "ong P.lS", "ongpls", " O.N.G - p l s "},
		{"Rémi", "Remi", "REMI", "rémi"},
		{"Corée du Nord", "coree du nord", "Coree-du-Nord", "coréedunord"},
		{"Biélorussie", "bielorussie"},
		{"Royaume-Uni", "royaume uni", "RoyaumeUni"},
		{"Ça c'est Noël", "ca cest noel"},
		{"Œdipe", "oedipe"},
		{"Agent 007", "agent007"},
	}
	for _, group := range same {
		want := NormalizeName(group[0])
		for _, name := range group[1:] {
			if got := NormalizeName(name); got != want {
				t.Errorf("NormalizeName(%q) = %q, want %q like %q", name, got, want, group[0])
			}
		}
	}

	if NormalizeName("Inde") == NormalizeName("Iran") {
		t.Error("different names normalize the same")
	}
	if got := NormalizeName(" .-' "); got != "" {
		t.Errorf("punctuation only: %q, want empty", got)
	}
}
