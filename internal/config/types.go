// Package config loads and validates the YAML game content: themes, setups and locales.
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Color is the kind of a slot in a setup.
type Color string

const (
	Blue   Color = "blue"
	Red    Color = "red"
	White  Color = "white"  // neutral that chooses its camp: blue, red or stays neutral
	Yellow Color = "yellow" // neutral with an asymmetric Rule
)

// Colors lists every color in display order.
var Colors = []Color{Blue, Red, White, Yellow}

func (c Color) valid() bool {
	switch c {
	case Blue, Red, White, Yellow:
		return true
	}
	return false
}

// Camp is a scoring team. White players end up in one of them; yellow players are Neutral.
type Camp string

const (
	CampBlue    Camp = "blue"
	CampRed     Camp = "red"
	CampNeutral Camp = "neutral"
)

// Camps lists every camp in display order.
var Camps = []Camp{CampBlue, CampRed, CampNeutral}

func (c Camp) valid() bool {
	switch c {
	case CampBlue, CampRed, CampNeutral:
		return true
	}
	return false
}

// Rule is the asymmetric scoring rule of a yellow slot.
type Rule string

const (
	AllyOfEveryone  Rule = "ally_of_everyone"
	EnemyOfEveryone Rule = "enemy_of_everyone"
)

// Rules lists every known rule.
var Rules = []Rule{AllyOfEveryone, EnemyOfEveryone}

func (r Rule) valid() bool {
	switch r {
	case AllyOfEveryone, EnemyOfEveryone:
		return true
	}
	return false
}

// SlotID identifies a slot of a setup, formatted as "<color>-<n>" with n starting at 1, e.g. "blue-2".
type SlotID string

// Parse splits the ID into its color and number.
func (id SlotID) Parse() (Color, int, error) {
	color, num, ok := strings.Cut(string(id), "-")
	if !ok {
		return "", 0, fmt.Errorf("slot %q: expected <color>-<number>", id)
	}
	if !Color(color).valid() {
		return "", 0, fmt.Errorf("slot %q: unknown color %q", id, color)
	}
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 || strconv.Itoa(n) != num {
		return "", 0, fmt.Errorf("slot %q: number must be a positive integer without leading zero", id)
	}
	return Color(color), n, nil
}
