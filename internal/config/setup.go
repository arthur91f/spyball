package config

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
)

const (
	MinPlayers = 6
	MaxPlayers = 24
)

// Setup is a theme-independent game configuration for an exact number of players:
// which slots exist and which slots each one has clues about.
type Setup struct {
	ID          string // file name without extension
	Name        string
	Description string
	Slots       []Slot // sorted by color then number
}

// Slot is one role of a setup, assigned to a player by the random draw.
type Slot struct {
	ID     SlotID
	Color  Color
	Number int      // 1-based rank within its color
	Rule   Rule     // yellow slots only
	Clues  []SlotID // slots whose sign this slot knows
}

type setupFile struct {
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description"`
	Slots       map[SlotID]setupFileSlot `yaml:"slots"`
}

type setupFileSlot struct {
	Rule  Rule     `yaml:"rule"`
	Clues []SlotID `yaml:"clues"`
}

// PlayerCount is the exact number of players the setup is made for.
func (s *Setup) PlayerCount() int {
	return len(s.Slots)
}

// Slot returns the slot with the given ID.
func (s *Setup) Slot(id SlotID) (Slot, bool) {
	for _, slot := range s.Slots {
		if slot.ID == id {
			return slot, true
		}
	}
	return Slot{}, false
}

// CountColor returns how many slots have the given color.
func (s *Setup) CountColor(c Color) int {
	n := 0
	for _, slot := range s.Slots {
		if slot.Color == c {
			n++
		}
	}
	return n
}

func newSetup(id string, f setupFile) (*Setup, error) {
	if f.Name == "" {
		return nil, errors.New("missing name")
	}

	s := &Setup{ID: id, Name: f.Name, Description: f.Description}
	for slotID, raw := range f.Slots {
		color, n, err := slotID.Parse()
		if err != nil {
			return nil, err
		}
		s.Slots = append(s.Slots, Slot{ID: slotID, Color: color, Number: n, Rule: raw.Rule, Clues: raw.Clues})
	}
	slices.SortFunc(s.Slots, func(a, b Slot) int {
		return cmp.Or(
			cmp.Compare(slices.Index(Colors, a.Color), slices.Index(Colors, b.Color)),
			cmp.Compare(a.Number, b.Number),
		)
	})

	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Setup) validate() error {
	var errs []error

	if n := s.PlayerCount(); n < MinPlayers || n > MaxPlayers {
		errs = append(errs, fmt.Errorf("%d slots, want between %d and %d", n, MinPlayers, MaxPlayers))
	}
	if b, r := s.CountColor(Blue), s.CountColor(Red); b != r {
		errs = append(errs, fmt.Errorf("%d blue slots but %d red slots, want the same number", b, r))
	}

	// Slots are sorted, so numbers of each color must read 1, 2, 3…
	next := map[Color]int{}
	for _, slot := range s.Slots {
		next[slot.Color]++
		if slot.Number != next[slot.Color] {
			errs = append(errs, fmt.Errorf("slot %s: %s slots must be numbered 1 to n without gaps", slot.ID, slot.Color))
			next[slot.Color] = slot.Number
		}

		switch {
		case slot.Color == Yellow && !slot.Rule.valid():
			errs = append(errs, fmt.Errorf("slot %s: yellow slots need a rule among %v, got %q", slot.ID, Rules, slot.Rule))
		case slot.Color != Yellow && slot.Rule != "":
			errs = append(errs, fmt.Errorf("slot %s: only yellow slots can have a rule", slot.ID))
		}

		seen := map[SlotID]bool{}
		for _, clue := range slot.Clues {
			switch _, exists := s.Slot(clue); {
			case clue == slot.ID:
				errs = append(errs, fmt.Errorf("slot %s: clue about itself", slot.ID))
			case !exists:
				errs = append(errs, fmt.Errorf("slot %s: clue about unknown slot %s", slot.ID, clue))
			case seen[clue]:
				errs = append(errs, fmt.Errorf("slot %s: duplicate clue %s", slot.ID, clue))
			}
			seen[clue] = true
		}
	}

	return errors.Join(errs...)
}
