package config

import (
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
	for color, want := range map[Color]int{Blue: 4, Red: 4, White: 2, Yellow: 2} {
		if got := setup.CountColor(color); got != want {
			t.Errorf("CountColor(%s) = %d, want %d", color, got, want)
		}
	}

	cast, err := theme.Cast(setup)
	if err != nil {
		t.Fatalf("Cast: %v", err)
	}
	for slot, want := range map[SlotID]string{"blue-1": "USA", "red-1": "Russie", "white-2": "Brésil", "yellow-1": "ONG P.L.S.", "yellow-2": "Anonymous"} {
		if got := cast[slot].Name; got != want {
			t.Errorf("cast[%s] = %q, want %q", slot, got, want)
		}
	}

	if !containsString(theme.Signs[Yellow], "faire chanter un autocrate") {
		t.Errorf("yellow signs not expanded: %q", theme.Signs[Yellow])
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
  neutral: {name: Neutres, member: neutre}
characters:
  blue: [{name: B1}, {name: B2}, {name: B3}]
  red: [{name: R1}, {name: R2}, {name: R3}]
  yellow: [{name: Ally, rule: ally_of_everyone}]
signs:
  blue: [b1, b2, b3]
  red: [r1, r2, r3]
  yellow: ["faire rire un {red}"]
`

const validSetup = `
name: Six
slots:
  blue-1: {clues: [blue-2]}
  blue-2: {clues: [blue-1]}
  blue-3: {}
  red-1: {}
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

	tests := []struct {
		name    string
		setup   string
		wantErr string
	}{
		{"unequal teams", "name: x\nslots: {blue-1: {}, blue-2: {}, blue-3: {}, red-1: {}, red-2: {}, white-1: {}}", "3 blue slots but 2 red"},
		{"too few players", "name: x\nslots: {blue-1: {}, red-1: {}}", "want between 6 and 24"},
		{"numbering gap", "name: x\nslots: {blue-1: {}, blue-3: {}, red-1: {}, red-2: {}, white-1: {}, white-2: {}}", "numbered 1 to n"},
		{"bad slot id", "name: x\nslots: {green-1: {}}", `unknown color "green"`},
		{"yellow without rule", "name: x\nslots: {blue-1: {}, blue-2: {}, red-1: {}, red-2: {}, white-1: {}, yellow-1: {}}", "yellow slots need a rule"},
		{"rule on blue", "name: x\nslots: {blue-1: {rule: ally_of_everyone}, blue-2: {}, red-1: {}, red-2: {}, white-1: {}, white-2: {}}", "only yellow slots"},
		{"unknown clue", "name: x\nslots: {blue-1: {clues: [red-9]}, blue-2: {}, red-1: {}, red-2: {}, white-1: {}, white-2: {}}", "unknown slot red-9"},
		{"self clue", "name: x\nslots: {blue-1: {clues: [blue-1]}, blue-2: {}, red-1: {}, red-2: {}, white-1: {}, white-2: {}}", "clue about itself"},
		{"typo in field", "name: x\nslots: {blue-1: {clue: [blue-2]}}", "field clue not found"},
		{"missing name", "slots: {blue-1: {}, blue-2: {}, blue-3: {}, red-1: {}, red-2: {}, red-3: {}}", "missing name"},
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
		{"unknown placeholder", func(s string) string { return strings.Replace(s, "{red}", "{green}", 1) }, "unknown placeholder {green}"},
		{"missing team", func(s string) string {
			return strings.Replace(s, "  neutral: {name: Neutres, member: neutre}\n", "", 1)
		}, "teams.neutral"},
		{"yellow without rule", func(s string) string { return strings.Replace(s, ", rule: ally_of_everyone", "", 1) }, "yellow characters need a rule"},
		{"duplicate sign", func(s string) string { return strings.Replace(s, "r1, r2", "r1, r1", 1) }, "duplicate sign"},
		{"white sign list", func(s string) string {
			return strings.Replace(s, "  yellow: [\"faire", "  white: [w1]\n  yellow: [\"faire", 1)
		}, "signs.white"},
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
		Name: "t",
		Teams: map[Camp]Team{
			CampBlue: {Name: "b", Member: "b"}, CampRed: {Name: "r", Member: "r"}, CampNeutral: {Name: "n", Member: "n"},
		},
		Characters: map[Color][]Character{
			Blue:   {{Name: "B1"}},
			Red:    {{Name: "R1"}},
			Yellow: {{Name: "Y1", Rule: AllyOfEveryone}},
		},
		Signs: map[Color][]string{Blue: {"b"}, Red: {"r"}, Yellow: {"y"}},
	})
	if err != nil {
		t.Fatalf("newTheme: %v", err)
	}

	setup := &Setup{ID: "s", Slots: []Slot{
		{ID: "blue-1", Color: Blue, Number: 1},
		{ID: "blue-2", Color: Blue, Number: 2},
		{ID: "red-1", Color: Red, Number: 1},
		{ID: "white-1", Color: White, Number: 1},
		{ID: "yellow-1", Color: Yellow, Number: 1, Rule: EnemyOfEveryone},
	}}
	_, err = theme.Cast(setup)
	for _, want := range []string{
		"only 1 blue characters",
		"only 0 white characters",
		"no more yellow character with rule enemy_of_everyone",
		"1 blue signs, setup s needs 3", // 2 blue slots + 1 white slot
		"1 red signs, setup s needs 2",  // 1 red slot + 1 white slot
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Cast err = %v, want containing %q", err, want)
		}
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
