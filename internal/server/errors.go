package server

import (
	"errors"
	"log/slog"

	"github.com/arthur91f/spyball/internal/game"
)

// errorKeys maps the errors a user can cause to the locale key of their message.
var errorKeys = []struct {
	err error
	key string
}{
	{game.ErrWrongPhase, "errors.wrong_phase"},
	{game.ErrUnknownPlayer, "errors.unknown_player"},
	{game.ErrEmptyName, "errors.empty_name"},
	{game.ErrDuplicateName, "errors.duplicate_name"},
	{game.ErrPlayerCount, "errors.player_count"},
	{game.ErrIncompatibleTheme, "errors.incompatible_theme"},
	{game.ErrCharacterRequired, "errors.character_required"},
	{game.ErrWrongCharacter, "errors.wrong_character"},
	{game.ErrInvalid, "errors.invalid"},
}

// describe turns an error into a message for the user. Unexpected errors are logged and get a generic message.
func (s *Server) describe(err error) string {
	key := "errors.generic"
	for _, e := range errorKeys {
		if errors.Is(err, e.err) {
			key = e.key
			break
		}
	}
	if key == "errors.generic" {
		slog.Error("unexpected error", "err", err)
	} else {
		slog.Info("request refused", "err", err)
	}

	msg, terr := s.locale.T(key)
	if terr != nil {
		slog.Error("missing error message", "err", terr)
		return err.Error()
	}
	return msg
}
