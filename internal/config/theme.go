package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Theme gives names, characters and signs to the color-based game. Adding a theme needs no code change.
type Theme struct {
	ID         string // directory name
	Name       string
	Story      string
	Teams      map[Color]Team
	Traitor    Role // themed name of the traitors
	Characters map[Color][]Character
	Signs      map[Color][]string // placeholders already replaced
	// Challenges are what traitors try to make other players do; placeholders already replaced.
	Challenges []string
	// Texts replaces texts of the locale, keyed like it ("rules.camps.balance").
	Texts map[string]string
}

// Team is the themed name of a color.
type Team struct {
	Name        string `yaml:"name"`
	Member      string `yaml:"member"` // noun for one member, substituted for {blue} or {red} in signs and challenges
	Description string `yaml:"description"`
}

// Role is the themed name of a special role.
type Role struct {
	Name        string `yaml:"name"`
	Team        string `yaml:"team"` // name of the camp the players with this role form
	Description string `yaml:"description"`
}

// Character is the themed identity given to a slot. Its name is secret and doubles as the player's password.
type Character struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type themeFile struct {
	Name       string                `yaml:"name"`
	Story      string                `yaml:"story"`
	Teams      map[Color]Team        `yaml:"teams"`
	Traitor    Role                  `yaml:"traitor"`
	Characters map[Color][]Character `yaml:"characters"`
	Signs      map[Color][]string    `yaml:"signs"`
	Challenges []string              `yaml:"challenges"`
	Texts      map[string]any        `yaml:"texts"`
}

var placeholderRe = regexp.MustCompile(`\{[^}]*\}`)

func newTheme(id string, f themeFile) (*Theme, error) {
	t := &Theme{
		ID:         id,
		Name:       f.Name,
		Story:      f.Story,
		Teams:      f.Teams,
		Traitor:    f.Traitor,
		Characters: f.Characters,
		Signs:      map[Color][]string{},
	}

	var errs []error
	if t.Name == "" {
		errs = append(errs, errors.New("missing name"))
	}
	for color := range f.Teams {
		if !color.valid() {
			errs = append(errs, fmt.Errorf("teams: unknown color %q, want blue or red", color))
		}
	}
	for _, color := range Colors {
		if team := f.Teams[color]; team.Name == "" || team.Member == "" {
			errs = append(errs, fmt.Errorf("teams.%s: name and member are required", color))
		}
	}
	if f.Traitor.Name == "" || f.Traitor.Team == "" {
		errs = append(errs, errors.New("traitor: name and team are required"))
	}

	names := map[string]bool{}
	for color, chars := range f.Characters {
		if !color.valid() {
			errs = append(errs, fmt.Errorf("characters: unknown color %q, want blue or red", color))
		}
		for i, c := range chars {
			where := fmt.Sprintf("characters.%s[%d]", color, i)
			key := NormalizeName(c.Name)
			switch {
			case key == "":
				errs = append(errs, fmt.Errorf("%s: missing name", where))
			case names[key]:
				errs = append(errs, fmt.Errorf("%s: duplicate character name %q", where, c.Name))
			}
			names[key] = true
		}
	}

	seen := map[string]bool{}
	for color, signs := range f.Signs {
		if !color.valid() {
			errs = append(errs, fmt.Errorf("signs: unknown color %q, want blue or red", color))
		}
		t.Signs[color] = t.textList(fmt.Sprintf("signs.%s", color), "sign", signs, seen, &errs)
	}
	t.Challenges = t.textList("challenges", "challenge", f.Challenges, map[string]bool{}, &errs)

	if f.Texts != nil {
		t.Texts = map[string]string{}
		if err := flatten(t.Texts, "", f.Texts); err != nil {
			errs = append(errs, fmt.Errorf("texts: %w", err))
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return t, nil
}

// textList validates a list of signs or challenges and returns them with placeholders replaced.
// seen holds the texts already used, to reject duplicates across lists.
func (t *Theme) textList(key, what string, texts []string, seen map[string]bool, errs *[]error) []string {
	var list []string
	for i, text := range texts {
		where := fmt.Sprintf("%s[%d]", key, i)
		expanded := t.expand(text)
		switch {
		case strings.TrimSpace(text) == "":
			*errs = append(*errs, fmt.Errorf("%s: empty %s", where, what))
		case seen[expanded]:
			*errs = append(*errs, fmt.Errorf("%s: duplicate %s %q", where, what, text))
		}
		if p := placeholderRe.FindString(expanded); p != "" {
			*errs = append(*errs, fmt.Errorf("%s: unknown placeholder %s, want {blue} or {red}", where, p))
		}
		seen[expanded] = true
		list = append(list, expanded)
	}
	return list
}

// expand replaces {blue} and {red} with the member noun of the matching team.
func (t *Theme) expand(s string) string {
	for _, color := range Colors {
		s = strings.ReplaceAll(s, "{"+string(color)+"}", t.Teams[color].Member)
	}
	return s
}

// Cast returns the character of every slot of the setup: the n-th slot of a color gets the n-th character
// of that color. It fails if the theme lacks characters, signs or challenges for the setup.
func (t *Theme) Cast(s *Setup) (map[SlotID]Character, error) {
	var errs []error
	cast := make(map[SlotID]Character, len(s.Slots))
	for _, slot := range s.Slots {
		chars := t.Characters[slot.Color]
		if slot.Number > len(chars) {
			errs = append(errs, fmt.Errorf("slot %s: theme %s has only %d %s characters", slot.ID, t.ID, len(chars), slot.Color))
			continue
		}
		cast[slot.ID] = chars[slot.Number-1]
	}

	for _, color := range Colors {
		if have, need := len(t.Signs[color]), s.CountColor(color); need > have {
			errs = append(errs, fmt.Errorf("theme %s has %d %s signs, setup %s needs %d", t.ID, have, color, s.ID, need))
		}
	}
	// Every traitor gets the same challenges.
	if have, need := len(t.Challenges), s.Challenges; need > have {
		errs = append(errs, fmt.Errorf("theme %s has %d challenges, setup %s needs %d", t.ID, have, s.ID, need))
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return cast, nil
}

// Text returns the text of a locale key: the theme's own if it has one, else the locale's. t may be nil.
func (t *Theme) Text(l *Locale, key string) (string, error) {
	if t != nil {
		if text, ok := t.Texts[key]; ok {
			return text, nil
		}
	}
	return l.T(key)
}
