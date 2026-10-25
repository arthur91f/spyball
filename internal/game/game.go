// Package game holds the state and rules of a Spyball game: players, random draw, guesses and scores.
// It does no I/O; callers serialize the Game (it is JSON-friendly) and guard it against concurrent access.
package game

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/arthur91f/spyball/internal/config"
)

// Phase is the stage of the game.
type Phase string

const (
	PhaseSetup   Phase = "setup"   // the admin enters players
	PhaseRunning Phase = "running" // roles are drawn, players enter guesses
	PhaseClosed  Phase = "closed"  // guesses are frozen, results are public
)

// PlayerID identifies a player within a game.
type PlayerID string

// Guess is what a player thinks of another player.
type Guess string

const (
	GuessNone  Guess = ""
	GuessAlly  Guess = "ally"
	GuessEnemy Guess = "enemy"
)

// Errors callers may want to tell apart, e.g. to show a translated message. Returned errors wrap them.
var (
	ErrWrongPhase        = errors.New("not allowed at this stage of the game")
	ErrUnknownPlayer     = errors.New("unknown player")
	ErrEmptyName         = errors.New("empty player name")
	ErrDuplicateName     = errors.New("player name already used")
	ErrPlayerCount       = errors.New("player count does not match the setup")
	ErrIncompatibleTheme = errors.New("theme cannot be used with this setup")
	ErrInvalid           = errors.New("invalid request")
	ErrCharacterRequired = errors.New("character name required to log in again")
	ErrWrongCharacter    = errors.New("wrong character name")
)

// Game is the whole state of one game. Everything needed to play and score is copied into it
// at draw time, so it stays consistent even if the YAML content changes afterwards.
type Game struct {
	Phase        Phase         `json:"phase"`
	ThemeID      string        `json:"theme_id,omitempty"`
	SetupID      string        `json:"setup_id,omitempty"`
	Players      []*Player     `json:"players"`
	ManualPoints []ManualPoint `json:"manual_points,omitempty"`
	LastID       int           `json:"last_id"`
}

// Player is a guest and, once roles are drawn, their secret role.
type Player struct {
	ID   PlayerID `json:"id"`
	Name string   `json:"name"`

	// Set by the draw.
	Slot      config.SlotID `json:"slot,omitempty"`
	Color     config.Color  `json:"color,omitempty"`
	Rule      config.Rule   `json:"rule,omitempty"`
	Character string        `json:"character,omitempty"`
	// Description is the themed text of the character, if any.
	Description string `json:"description,omitempty"`
	// Signs has one sign per entry of Color.SignColors(), in the same order.
	Signs []string `json:"signs,omitempty"`
	// Clues are the players whose sign this player knows.
	Clues []PlayerID `json:"clues,omitempty"`

	// Set while playing.
	// Revealed is true once the player has logged in and seen their character; from then on,
	// logging in requires the character name too.
	Revealed bool               `json:"revealed,omitempty"`
	Choice   config.Camp        `json:"choice,omitempty"` // white players only; empty means neutral
	Guesses  map[PlayerID]Guess `json:"guesses,omitempty"`
}

// ManualPoint is a bonus or penalty given by the admin when results are shown
// (successful yellow challenge, public denunciation…).
type ManualPoint struct {
	Player PlayerID `json:"player"`
	Points int      `json:"points"`
	Reason string   `json:"reason,omitempty"`
}

// New returns an empty game in the setup phase.
func New() *Game {
	return &Game{Phase: PhaseSetup}
}

// Player returns the player with the given ID.
func (g *Game) Player(id PlayerID) (*Player, error) {
	for _, p := range g.Players {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownPlayer, id)
}

// PlayerByName returns the player with the given name, compared with config.NormalizeName.
func (g *Game) PlayerByName(name string) (*Player, bool) {
	key := config.NormalizeName(name)
	for _, p := range g.Players {
		if config.NormalizeName(p.Name) == key {
			return p, true
		}
	}
	return nil, false
}

// AddPlayer registers a guest. Names must be unique (compared with config.NormalizeName, so "Rémi"
// and "remi" are the same) and are only accepted before the draw.
func (g *Game) AddPlayer(name string) (*Player, error) {
	if err := g.requirePhase(PhaseSetup); err != nil {
		return nil, err
	}
	name = strings.Join(strings.Fields(name), " ")
	if config.NormalizeName(name) == "" { // e.g. only punctuation: could not be told apart from another name
		return nil, ErrEmptyName
	}
	if _, exists := g.PlayerByName(name); exists {
		return nil, fmt.Errorf("%w: %q", ErrDuplicateName, name)
	}

	g.LastID++
	p := &Player{ID: PlayerID("p" + strconv.Itoa(g.LastID)), Name: name}
	g.Players = append(g.Players, p)
	return p, nil
}

// RemovePlayer removes a guest before the draw.
func (g *Game) RemovePlayer(id PlayerID) error {
	if err := g.requirePhase(PhaseSetup); err != nil {
		return err
	}
	for i, p := range g.Players {
		if p.ID == id {
			g.Players = append(g.Players[:i], g.Players[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrUnknownPlayer, id)
}

// SetGuess records what a player thinks of another one. GuessNone clears it.
func (g *Game) SetGuess(from, about PlayerID, guess Guess) error {
	if err := g.requirePhase(PhaseRunning); err != nil {
		return err
	}
	p, err := g.Player(from)
	if err != nil {
		return err
	}
	if _, err := g.Player(about); err != nil {
		return err
	}
	if from == about {
		return fmt.Errorf("%w: cannot guess about yourself", ErrInvalid)
	}

	switch guess {
	case GuessNone:
		delete(p.Guesses, about)
	case GuessAlly, GuessEnemy:
		if p.Guesses == nil {
			p.Guesses = map[PlayerID]Guess{}
		}
		p.Guesses[about] = guess
	default:
		return fmt.Errorf("%w: unknown guess %q", ErrInvalid, guess)
	}
	return nil
}

// ChooseCamp records the camp a white player joins. It can be changed until the game is closed.
func (g *Game) ChooseCamp(id PlayerID, camp config.Camp) error {
	if err := g.requirePhase(PhaseRunning); err != nil {
		return err
	}
	p, err := g.Player(id)
	if err != nil {
		return err
	}
	if p.Color != config.White {
		return fmt.Errorf("%w: only white players choose their camp", ErrInvalid)
	}
	switch camp {
	case config.CampBlue, config.CampRed, config.CampNeutral:
		p.Choice = camp
	default:
		return fmt.Errorf("%w: unknown camp %q", ErrInvalid, camp)
	}
	return nil
}

// Login identifies a player by name. The first time, the name is enough and the character is revealed;
// afterwards the character name is required, compared with config.NormalizeName.
// Players can log in while the game runs and after it is closed.
func (g *Game) Login(name, character string) (*Player, error) {
	if g.Phase == PhaseSetup {
		return nil, fmt.Errorf("%w: roles are not drawn yet", ErrWrongPhase)
	}
	p, ok := g.PlayerByName(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownPlayer, name)
	}
	if !p.Revealed {
		p.Revealed = true
		return p, nil
	}
	switch {
	case config.NormalizeName(character) == "":
		return nil, ErrCharacterRequired
	case config.NormalizeName(character) != config.NormalizeName(p.Character):
		return nil, ErrWrongCharacter
	}
	return p, nil
}

// ResetLogin lets a player who forgot their character log in again with their name only.
func (g *Game) ResetLogin(id PlayerID) error {
	if err := g.requirePhase(PhaseRunning); err != nil {
		return err
	}
	p, err := g.Player(id)
	if err != nil {
		return err
	}
	p.Revealed = false
	return nil
}

// Close freezes guesses and camp choices; results become available.
func (g *Game) Close() error {
	if err := g.requirePhase(PhaseRunning); err != nil {
		return err
	}
	g.Phase = PhaseClosed
	return nil
}

// AddManualPoints gives a bonus (or a penalty if negative) to a player while results are shown.
func (g *Game) AddManualPoints(id PlayerID, points int, reason string) error {
	if err := g.requirePhase(PhaseClosed); err != nil {
		return err
	}
	if _, err := g.Player(id); err != nil {
		return err
	}
	if points == 0 {
		return fmt.Errorf("%w: zero points", ErrInvalid)
	}
	g.ManualPoints = append(g.ManualPoints, ManualPoint{Player: id, Points: points, Reason: strings.TrimSpace(reason)})
	return nil
}

// RemoveManualPoints cancels the i-th manual point entry, to fix a mistake.
func (g *Game) RemoveManualPoints(i int) error {
	if err := g.requirePhase(PhaseClosed); err != nil {
		return err
	}
	if i < 0 || i >= len(g.ManualPoints) {
		return fmt.Errorf("%w: no manual points entry %d", ErrInvalid, i)
	}
	g.ManualPoints = append(g.ManualPoints[:i], g.ManualPoints[i+1:]...)
	return nil
}

func (g *Game) requirePhase(want Phase) error {
	if g.Phase != want {
		return fmt.Errorf("%w: game is %s, want %s", ErrWrongPhase, g.Phase, want)
	}
	return nil
}
