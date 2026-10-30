// Package config loads and validates the YAML game content: themes, setups and locales.
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Color is the team of a slot in a setup. Traitors are ordinary members of their color.
type Color string

const (
	Blue Color = "blue"
	Red  Color = "red"
)

// Colors lists every color in display order.
var Colors = []Color{Blue, Red}

func (c Color) valid() bool {
	switch c {
	case Blue, Red:
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
		return "", 0, fmt.Errorf("slot %q: unknown color %q, want blue or red", id, color)
	}
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 || strconv.Itoa(n) != num {
		return "", 0, fmt.Errorf("slot %q: number must be a positive integer without leading zero", id)
	}
	return Color(color), n, nil
}
