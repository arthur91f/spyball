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
	Teams      map[Camp]Team
	Characters map[Color][]Character
	Signs      map[Color][]string // blue, red and yellow lists (see Color.SignColors); placeholders already replaced
}

// Team is the themed name of a camp.
type Team struct {
	Name        string `yaml:"name"`
	Member      string `yaml:"member"` // noun for one member, substituted for {camp} in signs
	Description string `yaml:"description"`
}

// Character is the themed identity given to a slot. Its name is secret and doubles as the player's password.
type Character struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Rule        Rule   `yaml:"rule"` // yellow characters only: the slot rule it can fill
}

type themeFile struct {
	Name       string                `yaml:"name"`
	Story      string                `yaml:"story"`
	Teams      map[Camp]Team         `yaml:"teams"`
	Characters map[Color][]Character `yaml:"characters"`
	Signs      map[Color][]string    `yaml:"signs"`
}

var placeholderRe = regexp.MustCompile(`\{[^}]*\}`)

func newTheme(id string, f themeFile) (*Theme, error) {
	t := &Theme{
		ID:         id,
		Name:       f.Name,
		Story:      f.Story,
		Teams:      f.Teams,
		Characters: f.Characters,
		Signs:      map[Color][]string{},
	}

	var errs []error
	if t.Name == "" {
		errs = append(errs, errors.New("missing name"))
	}
	for camp := range f.Teams {
		if !camp.valid() {
			errs = append(errs, fmt.Errorf("teams: unknown camp %q", camp))
		}
	}
	for _, camp := range Camps {
		if team := f.Teams[camp]; team.Name == "" || team.Member == "" {
			errs = append(errs, fmt.Errorf("teams.%s: name and member are required", camp))
		}
	}

	names := map[string]bool{}
	for color, chars := range f.Characters {
		if !color.valid() {
			errs = append(errs, fmt.Errorf("characters: unknown color %q", color))
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

			switch {
			case color == Yellow && !c.Rule.valid():
				errs = append(errs, fmt.Errorf("%s: yellow characters need a rule among %v, got %q", where, Rules, c.Rule))
			case color != Yellow && c.Rule != "":
				errs = append(errs, fmt.Errorf("%s: only yellow characters can have a rule", where))
			}
		}
	}

	seen := map[string]bool{}
	for color, signs := range f.Signs {
		switch {
		case color == White:
			errs = append(errs, errors.New("signs.white: white players draw from the blue and red lists, remove this list"))
		case !color.valid():
			errs = append(errs, fmt.Errorf("signs: unknown color %q", color))
		}
		for i, sign := range signs {
			where := fmt.Sprintf("signs.%s[%d]", color, i)
			expanded := t.expand(sign)
			switch {
			case strings.TrimSpace(sign) == "":
				errs = append(errs, fmt.Errorf("%s: empty sign", where))
			case seen[expanded]:
				errs = append(errs, fmt.Errorf("%s: duplicate sign %q", where, sign))
			}
			if p := placeholderRe.FindString(expanded); p != "" {
				errs = append(errs, fmt.Errorf("%s: unknown placeholder %s, want one of {blue}, {red}, {neutral}", where, p))
			}
			seen[expanded] = true
			t.Signs[color] = append(t.Signs[color], expanded)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return t, nil
}

// expand replaces {blue}, {red} and {neutral} with the member noun of the matching team.
func (t *Theme) expand(s string) string {
	for _, camp := range Camps {
		s = strings.ReplaceAll(s, "{"+string(camp)+"}", t.Teams[camp].Member)
	}
	return s
}

// Cast returns the character of every slot of the setup: the n-th slot of a color gets the n-th character
// of that color, except yellow slots which get the characters matching their rule, in order.
// It fails if the theme lacks characters or signs for the setup.
func (t *Theme) Cast(s *Setup) (map[SlotID]Character, error) {
	var errs []error
	cast := make(map[SlotID]Character, len(s.Slots))
	usedByRule := map[Rule]int{}

	for _, slot := range s.Slots {
		if slot.Color == Yellow {
			candidates := t.charactersWithRule(slot.Rule)
			if usedByRule[slot.Rule] >= len(candidates) {
				errs = append(errs, fmt.Errorf("slot %s: theme %s has no more yellow character with rule %s", slot.ID, t.ID, slot.Rule))
				continue
			}
			cast[slot.ID] = candidates[usedByRule[slot.Rule]]
			usedByRule[slot.Rule]++
			continue
		}

		chars := t.Characters[slot.Color]
		if slot.Number > len(chars) {
			errs = append(errs, fmt.Errorf("slot %s: theme %s has only %d %s characters", slot.ID, t.ID, len(chars), slot.Color))
			continue
		}
		cast[slot.ID] = chars[slot.Number-1]
	}

	need := map[Color]int{}
	for _, slot := range s.Slots {
		for _, c := range slot.Color.SignColors() {
			need[c]++
		}
	}
	for _, color := range Colors {
		if have := len(t.Signs[color]); need[color] > have {
			errs = append(errs, fmt.Errorf("theme %s has %d %s signs, setup %s needs %d", t.ID, have, color, s.ID, need[color]))
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return cast, nil
}

func (t *Theme) charactersWithRule(r Rule) []Character {
	var chars []Character
	for _, c := range t.Characters[Yellow] {
		if c.Rule == r {
			chars = append(chars, c)
		}
	}
	return chars
}

// NormalizeName is the form under which character and player names are compared: trimmed, case-insensitive.
func NormalizeName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}
