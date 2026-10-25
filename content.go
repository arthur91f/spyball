// Package spyball embeds the default game content: themes, setups and locales.
package spyball

import "embed"

// Content holds the themes/, setups/ and locales/ directories.
//
//go:embed themes setups locales
var Content embed.FS
