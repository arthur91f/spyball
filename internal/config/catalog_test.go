package config

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arthur91f/spyball"
)

func TestLoadEmbeddedContent(t *testing.T) {
	cat, err := Load(spyball.Content)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	theme := cat.Themes["ball-of-nations"]
	if theme == nil {
		t.Fatal("theme ball-of-nations not loaded")
	}
	setup := cat.Setups["12-classic"]
	if setup == nil {
		t.Fatal("setup 12-classic not loaded")
	}

	if got := setup.PlayerCount(); got != 12 {
		t.Errorf("PlayerCount = %d, want 12", got)
	}
	for _, color := range Colors {
		if got := setup.CountColor(color); got != 6 {
			t.Errorf("CountColor(%s) = %d, want 6", color, got)
		}
		if got := setup.CountTraitors(color); got != 1 {
			t.Errorf("CountTraitors(%s) = %d, want 1", color, got)
		}
	}
	if setup.Challenges < 1 {
		t.Errorf("Challenges = %d, want at least 1", setup.Challenges)
	}

	cast, err := theme.Cast(setup)
	if err != nil {
		t.Fatalf("Cast: %v", err)
	}
	for slot, want := range map[SlotID]string{"blue-1": "USA", "red-1": "Russie", "red-6": "Cuba"} {
		if got := cast[slot].Name; got != want {
			t.Errorf("cast[%s] = %q, want %q", slot, got, want)
		}
	}

	if len(theme.Challenges) < setup.Challenges {
		t.Errorf("%d challenges, setup needs %d", len(theme.Challenges), setup.Challenges)
	}

	if got := cat.SetupsFor(theme, 12); len(got) != 1 || got[0].ID != "12-classic" {
		t.Errorf("SetupsFor(12) = %v, want [12-classic]", got)
	}
	if got := cat.SetupsFor(theme, 7); len(got) != 0 {
		t.Errorf("SetupsFor(7) = %v, want none", got)
	}

	fr := cat.Locales["fr"]
	if fr == nil {
		t.Fatal("locale fr not loaded")
	}
	if _, err := fr.T("home.tagline"); err != nil {
		t.Error(err)
	}
	if _, err := fr.T("does.not.exist"); err == nil {
		t.Error("T on a missing key: want error")
	}
}

const validTheme = `
name: Test
teams:
  blue: {name: Bleus, member: bleu}
  red: {name: Rouges, member: rouge}
traitor: {name: Traître, team: Traîtres}
characters:
  blue: [{name: B1}, {name: B2}, {name: B3}]
  red: [{name: R1}, {name: R2}, {name: R3}]
signs:
  blue: [b1, b2, b3]
  red: [r1, r2, r3]
challenges: ["faire rire un {red}", "faire chanter un {blue}"]
`

const validSetup = `
name: Six
weight: 1
challenges: 1
slots:
  blue-1: {traitor: true, clues: [blue-2]}
  blue-2: {clues: [blue-1]}
  blue-3: {}
  red-1: {traitor: true}
  red-2: {}
  red-3: {}
`

const validLocale = "app:\n  name: Test\n"

func loadWith(t *testing.T, overrides map[string]string) error {
	t.Helper()
	files := map[string]string{
		"themes/test/theme.yaml": validTheme,
		"setups/six.yaml":        validSetup,
		"locales/fr.yaml":        validLocale,
	}
	for name, content := range overrides {
		files[name] = content
	}
	fsys := fstest.MapFS{}
	for name, content := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	_, err := Load(fsys)
	return err
}

func TestLoadRejectsInvalidSetups(t *testing.T) {
	if err := loadWith(t, nil); err != nil {
		t.Fatalf("valid fixture: %v", err)
	}
	// Teams of different sizes, traitors in one team only: accepted.
	uneven := "name: x\nweight: 0\nchallenges: 1\nslots: {blue-1: {traitor: true}, blue-2: {traitor: true}, blue-3: {}, blue-4: {}, red-1: {}, red-2: {}, red-3: {}}"
	if err := loadWith(t, map[string]string{"setups/seven.yaml": uneven}); err != nil {
		t.Errorf("uneven setup: %v", err)
	}

	tests := []struct {
		name    string
		setup   string
		wantErr string
	}{
		{"too few players", "name: x\nweight: 1\nslots: {blue-1: {}, red-1: {}}", "want between 6 and 24"},
		{"numbering gap", "name: x\nweight: 1\nslots: {blue-1: {}, blue-2: {}, blue-4: {}, red-1: {}, red-2: {}, red-3: {}}", "numbered 1 to n"},
		{"bad slot id", "name: x\nweight: 1\nslots: {white-1: {}}", `unknown color "white"`},
		{"traitors without challenges", "name: x\nweight: 1\nslots: {blue-1: {traitor: true}, blue-2: {}, blue-3: {}, red-1: {traitor: true}, red-2: {}, red-3: {}}", "missing number of challenges"},
		{"challenges without traitors", "name: x\nweight: 1\nchallenges: 2\nslots: {blue-1: {}, blue-2: {}, blue-3: {}, red-1: {}, red-2: {}, red-3: {}}", "there is no traitor"},
		{"unknown clue", "name: x\nweight: 1\nslots: {blue-1: {clues: [red-9]}, blue-2: {}, blue-3: {}, red-1: {}, red-2: {}, red-3: {}}", "unknown slot red-9"},
		{"self clue", "name: x\nweight: 1\nslots: {blue-1: {clues: [blue-1]}, blue-2: {}, blue-3: {}, red-1: {}, red-2: {}, red-3: {}}", "clue about itself"},
		{"missing weight", "name: x\nslots: {blue-1: {}, blue-2: {}, blue-3: {}, red-1: {}, red-2: {}, red-3: {}}", "missing weight"},
		{"negative weight", "name: x\nweight: -1\nslots: {blue-1: {}, blue-2: {}, blue-3: {}, red-1: {}, red-2: {}, red-3: {}}", "want 0 or more"},
		{"typo in field", "name: x\nweight: 1\nslots: {blue-1: {clue: [blue-2]}}", "field clue not found"},
		{"missing name", "weight: 1\nslots: {blue-1: {}, blue-2: {}, blue-3: {}, red-1: {}, red-2: {}, red-3: {}}", "missing name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loadWith(t, map[string]string{"setups/six.yaml": tt.setup})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadRejectsInvalidThemes(t *testing.T) {
	tests := []struct {
		name    string
		edit    func(string) string
		wantErr string
	}{
		{"duplicate character", func(s string) string { return strings.Replace(s, "name: B2", "name: \" b1 \"", 1) }, "duplicate character name"},
		{"duplicate once normalized", func(s string) string { return strings.Replace(s, "name: B2", "name: \"b.1\"", 1) }, "duplicate character name"},
		{"unknown placeholder", func(s string) string { return strings.Replace(s, "{red}", "{green}", 1) }, "unknown placeholder {green}"},
		{"missing team", func(s string) string {
			return strings.Replace(s, "  red: {name: Rouges, member: rouge}\n", "", 1)
		}, "teams.red"},
		{"unknown team", func(s string) string {
			return strings.Replace(s, "teams:\n", "teams:\n  neutral: {name: Neutres, member: neutre}\n", 1)
		}, `teams: unknown color "neutral"`},
		{"missing traitor", func(s string) string {
			return strings.Replace(s, "traitor: {name: Traître, team: Traîtres}\n", "", 1)
		}, "traitor: name and team"},
		{"missing traitor team", func(s string) string { return strings.Replace(s, ", team: Traîtres", "", 1) }, "traitor: name and team"},
		{"white characters", func(s string) string {
			return strings.Replace(s, "  red: [{name: R1}", "  white: [{name: W1}]\n  red: [{name: R1}", 1)
		}, `characters: unknown color "white"`},
		{"duplicate sign", func(s string) string { return strings.Replace(s, "r1, r2", "r1, r1", 1) }, "duplicate sign"},
		{"duplicate challenge", func(s string) string {
			return strings.Replace(s, `"faire chanter un {blue}"`, `"faire rire un rouge"`, 1)
		}, "duplicate challenge"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loadWith(t, map[string]string{"themes/test/theme.yaml": tt.edit(validTheme)})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCast(t *testing.T) {
	theme, err := newTheme("test", themeFile{
		Name:       "t",
		Teams:      map[Color]Team{Blue: {Name: "b", Member: "b"}, Red: {Name: "r", Member: "r"}},
		Traitor:    Role{Name: "t", Team: "ts"},
		Characters: map[Color][]Character{Blue: {{Name: "B1"}}, Red: {{Name: "R1"}, {Name: "R2"}}},
		Signs:      map[Color][]string{Blue: {"b"}, Red: {"r1", "r2"}},
		Challenges: []string{"c1"},
	})
	if err != nil {
		t.Fatalf("newTheme: %v", err)
	}

	setup := &Setup{ID: "s", Challenges: 2, Slots: []Slot{
		{ID: "blue-1", Color: Blue, Number: 1, Traitor: true},
		{ID: "blue-2", Color: Blue, Number: 2},
		{ID: "red-1", Color: Red, Number: 1, Traitor: true},
		{ID: "red-2", Color: Red, Number: 2},
	}}
	_, err = theme.Cast(setup)
	for _, want := range []string{
		"only 1 blue characters",
		"1 blue signs, setup s needs 2",
		"1 challenges, setup s needs 2", // every traitor gets the same 2 challenges
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Cast err = %v, want containing %q", err, want)
		}
	}
}

func TestPickSetup(t *testing.T) {
	theme, err := newTheme("test", themeFile{
		Name:       "t",
		Teams:      map[Color]Team{Blue: {Name: "b", Member: "b"}, Red: {Name: "r", Member: "r"}},
		Traitor:    Role{Name: "t", Team: "ts"},
		Characters: map[Color][]Character{Blue: {{Name: "B1"}, {Name: "B2"}}, Red: {{Name: "R1"}}},
		Signs:      map[Color][]string{Blue: {"b1", "b2"}, Red: {"r1"}},
	})
	if err != nil {
		t.Fatalf("newTheme: %v", err)
	}
	slots := []Slot{{ID: "blue-1", Color: Blue, Number: 1}, {ID: "red-1", Color: Red, Number: 1}}
	cat := &Catalog{Setups: map[string]*Setup{
		"often":      {ID: "often", Weight: 3, Slots: slots},
		"rarely":     {ID: "rarely", Weight: 1, Slots: slots},
		"never":      {ID: "never", Weight: 0, Slots: slots},
		"three":      {ID: "three", Weight: 5, Slots: append(slices.Clone(slots), Slot{ID: "blue-2", Color: Blue, Number: 2})},
		"uncastable": {ID: "uncastable", Weight: 5, Slots: append(slices.Clone(slots), Slot{ID: "red-2", Color: Red, Number: 2})},
	}}

	rng := rand.New(rand.NewPCG(1, 2))
	counts := map[string]int{}
	for range 4000 {
		counts[cat.PickSetup(theme, 2, rng).ID]++
	}
	if len(counts) != 2 || counts["often"] < 2800 || counts["often"] > 3200 {
		t.Errorf("draws for 2 players = %v, want about 3000 often and 1000 rarely", counts)
	}
	if got := cat.PickSetup(theme, 3, rng); got == nil || got.ID != "three" {
		t.Errorf("PickSetup(3) = %v, want three (the uncastable one is skipped)", got)
	}
	if got := cat.PickSetup(theme, 4, rng); got != nil {
		t.Errorf("PickSetup(4) = %v, want nil", got.ID)
	}
	cat.Setups["often"].Weight, cat.Setups["rarely"].Weight = 0, 0
	if got := cat.PickSetup(theme, 2, rng); got != nil {
		t.Errorf("PickSetup with zero weights = %v, want nil", got.ID)
	}
}

func TestLocaleRejectsNonText(t *testing.T) {
	err := loadWith(t, map[string]string{"locales/fr.yaml": "app:\n  count: [1, 2]\n"})
	if err == nil || !strings.Contains(err.Error(), `key "app.count"`) {
		t.Errorf("err = %v, want error on app.count", err)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
