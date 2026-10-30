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
// which slots exist, which ones are traitors and which slots each one has clues about.
type Setup struct {
	ID          string // file name without extension
	Name        string
	Description string
	// Weight is the relative chance of this setup to be drawn among the setups for the same number
	// of players; 0 means never.
	Weight     int
	Challenges int    // number of challenges drawn for the game, the same for every traitor
	Slots      []Slot // sorted by color then number
}

// Slot is one role of a setup, assigned to a player by the random draw.
type Slot struct {
	ID      SlotID
	Color   Color
	Number  int      // 1-based rank within its color
	Traitor bool     // an ordinary member of its color, with challenges and a separate ranking
	Clues   []SlotID // slots whose sign this slot knows
}

type setupFile struct {
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description"`
	Weight      *int                     `yaml:"weight"` // a pointer to tell a missing weight from 0
	Challenges  int                      `yaml:"challenges"`
	Slots       map[SlotID]setupFileSlot `yaml:"slots"`
}

type setupFileSlot struct {
	Traitor bool     `yaml:"traitor"`
	Clues   []SlotID `yaml:"clues"`
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

// CountTraitors returns how many traitor slots have the given color.
func (s *Setup) CountTraitors(c Color) int {
	n := 0
	for _, slot := range s.Slots {
		if slot.Color == c && slot.Traitor {
			n++
		}
	}
	return n
}

func newSetup(id string, f setupFile) (*Setup, error) {
	if f.Name == "" {
		return nil, errors.New("missing name")
	}

	if f.Weight == nil {
		return nil, errors.New("missing weight")
	}
	s := &Setup{ID: id, Name: f.Name, Description: f.Description, Weight: *f.Weight, Challenges: f.Challenges}
	for slotID, raw := range f.Slots {
		color, n, err := slotID.Parse()
		if err != nil {
			return nil, err
		}
		s.Slots = append(s.Slots, Slot{ID: slotID, Color: color, Number: n, Traitor: raw.Traitor, Clues: raw.Clues})
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
	if s.Weight < 0 {
		errs = append(errs, fmt.Errorf("weight: %d, want 0 or more", s.Weight))
	}
	traitors := s.CountTraitors(Blue) + s.CountTraitors(Red)
	switch {
	case s.Challenges < 0:
		errs = append(errs, fmt.Errorf("challenges: %d, want a positive number", s.Challenges))
	case traitors > 0 && s.Challenges == 0:
		errs = append(errs, errors.New("challenges: missing number of challenges per traitor"))
	case traitors == 0 && s.Challenges > 0:
		errs = append(errs, errors.New("challenges: set but there is no traitor"))
	}

	// Slots are sorted, so numbers of each color must read 1, 2, 3…
	next := map[Color]int{}
	for _, slot := range s.Slots {
		next[slot.Color]++
		if slot.Number != next[slot.Color] {
			errs = append(errs, fmt.Errorf("slot %s: %s slots must be numbered 1 to n without gaps", slot.ID, slot.Color))
			next[slot.Color] = slot.Number
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
