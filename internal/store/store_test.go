package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/arthur91f/spyball"
	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/game"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "game.json")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, path
}

func snapshot(t *testing.T, s *Store) string {
	t.Helper()
	var out string
	_ = s.Read(func(g *game.Game) error {
		data, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		out = string(data)
		return nil
	})
	return out
}

func TestOpenMissingFileStartsNewGame(t *testing.T) {
	s, path := openTemp(t)
	_ = s.Read(func(g *game.Game) error {
		if g.Phase != game.PhaseSetup || len(g.Players) != 0 {
			t.Errorf("new game = %+v, want empty setup", g)
		}
		return nil
	})
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file created before any update: %v", err)
	}
}

func TestFullGameSurvivesRestart(t *testing.T) {
	cat, err := config.Load(spyball.Content)
	if err != nil {
		t.Fatal(err)
	}
	theme, setup := cat.Themes["ball-of-nations"], cat.Setups["12-classic"]

	s, path := openTemp(t)
	err = s.Update(func(g *game.Game) error {
		for i := range setup.PlayerCount() {
			if _, err := g.AddPlayer(fmt.Sprintf("Guest %d", i)); err != nil {
				return err
			}
		}
		if err := g.Draw(theme, setup, rand.New(rand.NewPCG(1, 2))); err != nil {
			return err
		}
		for _, p := range g.Players {
			if p.Color == config.White {
				_ = g.ChooseCamp(p.ID, config.CampRed)
			}
		}
		if err := g.SetGuess(g.Players[0].ID, g.Players[1].ID, game.GuessAlly); err != nil {
			return err
		}
		if err := g.Close(); err != nil {
			return err
		}
		return g.AddManualPoints(g.Players[2].ID, 2, "défi réussi")
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got, want := snapshot(t, reopened), snapshot(t, s); got != want {
		t.Errorf("reloaded game differs:\n got %s\nwant %s", got, want)
	}

	var before, after game.Results
	_ = s.Read(func(g *game.Game) error { before = g.Score(); return nil })
	_ = reopened.Read(func(g *game.Game) error { after = g.Score(); return nil })
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Errorf("scores differ after reload: %v vs %v", before, after)
	}
}

func TestFailedUpdateChangesNothing(t *testing.T) {
	s, path := openTemp(t)
	if err := s.Update(func(g *game.Game) error { _, err := g.AddPlayer("Alice"); return err }); err != nil {
		t.Fatal(err)
	}
	fileBefore, _ := os.ReadFile(path)
	memBefore := snapshot(t, s)

	boom := errors.New("boom")
	err := s.Update(func(g *game.Game) error {
		_, _ = g.AddPlayer("Bob") // half-done change, then failure
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}

	if got := snapshot(t, s); got != memBefore {
		t.Errorf("memory changed by failed update: %s", got)
	}
	if got, _ := os.ReadFile(path); string(got) != string(fileBefore) {
		t.Errorf("file changed by failed update: %s", got)
	}
}

func TestFailedSaveChangesNothing(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "missing-dir", "game.json"))
	if err != nil {
		t.Fatal(err)
	}
	err = s.Update(func(g *game.Game) error { _, err := g.AddPlayer("Alice"); return err })
	if err == nil {
		t.Fatal("save into a missing directory: want error")
	}
	_ = s.Read(func(g *game.Game) error {
		if len(g.Players) != 0 {
			t.Error("memory changed although the save failed")
		}
		return nil
	})
}

func TestCorruptFileIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("corrupt file: want error")
	}
	if data, _ := os.ReadFile(path); string(data) != "{not json" {
		t.Error("corrupt file was overwritten")
	}
}

func TestNoTempFilesLeft(t *testing.T) {
	s, path := openTemp(t)
	for _, name := range []string{"A", "B", "C"} {
		if err := s.Update(func(g *game.Game) error { _, err := g.AddPlayer(name); return err }); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 || entries[0].Name() != "game.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want only game.json", names)
	}
}

func TestConcurrentUpdates(t *testing.T) {
	s, path := openTemp(t)
	const n = 50

	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			if err := s.Update(func(g *game.Game) error {
				_, err := g.AddPlayer(fmt.Sprintf("Guest %d", i))
				return err
			}); err != nil {
				t.Error(err)
			}
		})
		wg.Go(func() {
			_ = s.Read(func(g *game.Game) error { _ = len(g.Players); return nil })
		})
	}
	wg.Wait()

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = reopened.Read(func(g *game.Game) error {
		if len(g.Players) != n {
			t.Errorf("%d players saved, want %d", len(g.Players), n)
		}
		return nil
	})
}
