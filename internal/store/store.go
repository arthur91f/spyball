// Package store keeps the single game of the server in memory, persisted to a JSON file,
// and serializes access to it.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/arthur91f/spyball/internal/game"
)

// Store guards the current game. Reads run concurrently; updates are exclusive and saved before
// they become visible, so the file and the memory never disagree.
type Store struct {
	mu   sync.RWMutex
	path string
	game *game.Game
}

// Open loads the game saved at path, or starts a new one if the file does not exist yet.
// A file that cannot be decoded is an error: it is never silently overwritten.
func Open(path string) (*Store, error) {
	s := &Store{path: path}

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		s.game = game.New()
		return s, nil
	case err != nil:
		return nil, err
	}

	var g game.Game
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	s.game = &g
	return s, nil
}

// Path is the file the game is saved to.
func (s *Store) Path() string {
	return s.path
}

// Read calls fn with the current game. fn must not modify it nor keep it after returning.
func (s *Store) Read(fn func(g *game.Game) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fn(s.game)
}

// Update calls fn with a copy of the game and, if fn succeeds, saves the copy and makes it current.
// If fn or the save fails, the current game is left untouched.
func (s *Store) Update(fn func(g *game.Game) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(s.game)
	if err != nil {
		return err
	}
	var next game.Game
	if err := json.Unmarshal(data, &next); err != nil {
		return err
	}

	if err := fn(&next); err != nil {
		return err
	}
	if err := s.save(&next); err != nil {
		return fmt.Errorf("save game: %w", err)
	}
	s.game = &next
	return nil
}

// save writes the game to a temporary file then renames it over the previous one,
// so a crash never leaves a half-written file.
func (s *Store) save(g *game.Game) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return err
	}

	// Persist the rename itself.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}
