package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"path"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Catalog is all the game content available to the server.
type Catalog struct {
	Themes  map[string]*Theme
	Setups  map[string]*Setup
	Locales map[string]*Locale
}

// Load reads and validates themes/<id>/theme.yaml, setups/<id>.yaml and locales/<lang>.yaml from fsys.
// Every problem found is reported, not just the first one.
func Load(fsys fs.FS) (*Catalog, error) {
	c := &Catalog{Themes: map[string]*Theme{}, Setups: map[string]*Setup{}, Locales: map[string]*Locale{}}
	var errs []error

	themeFiles, _ := fs.Glob(fsys, "themes/*/theme.yaml")
	for _, file := range themeFiles {
		id := path.Base(path.Dir(file))
		var f themeFile
		err := decodeFile(fsys, file, &f)
		if err == nil {
			c.Themes[id], err = newTheme(id, f)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
		}
	}

	setupFiles, _ := fs.Glob(fsys, "setups/*.yaml")
	for _, file := range setupFiles {
		id := strings.TrimSuffix(path.Base(file), ".yaml")
		var f setupFile
		err := decodeFile(fsys, file, &f)
		if err == nil {
			c.Setups[id], err = newSetup(id, f)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
		}
	}

	localeFiles, _ := fs.Glob(fsys, "locales/*.yaml")
	for _, file := range localeFiles {
		lang := strings.TrimSuffix(path.Base(file), ".yaml")
		var tree map[string]any
		err := decodeFile(fsys, file, &tree)
		if err == nil {
			c.Locales[lang], err = newLocale(lang, tree)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
		}
	}

	if len(c.Themes) == 0 {
		errs = append(errs, errors.New("no theme found in themes/*/theme.yaml"))
	}
	if len(c.Setups) == 0 {
		errs = append(errs, errors.New("no setup found in setups/*.yaml"))
	}
	if len(c.Locales) == 0 {
		errs = append(errs, errors.New("no locale found in locales/*.yaml"))
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return c, nil
}

// SetupsFor returns the setups made for exactly n players that the theme can cast, sorted by ID.
func (c *Catalog) SetupsFor(theme *Theme, n int) []*Setup {
	var setups []*Setup
	for _, s := range c.Setups {
		if s.PlayerCount() != n {
			continue
		}
		if _, err := theme.Cast(s); err == nil {
			setups = append(setups, s)
		}
	}
	sort.Slice(setups, func(i, j int) bool { return setups[i].ID < setups[j].ID })
	return setups
}

// PickSetup draws one of the setups for exactly n players that the theme can cast, with a chance
// proportional to its weight. It returns nil if there is none (or if they all weigh 0).
func (c *Catalog) PickSetup(theme *Theme, n int, rng *rand.Rand) *Setup {
	setups := c.SetupsFor(theme, n) // sorted, so the result only depends on the rng
	total := 0
	for _, s := range setups {
		total += s.Weight
	}
	if total == 0 {
		return nil
	}
	r := rng.IntN(total)
	for _, s := range setups {
		if r < s.Weight {
			return s
		}
		r -= s.Weight
	}
	panic("unreachable")
}

// decodeFile decodes YAML strictly: unknown fields are errors, to catch typos in hand-written files.
func decodeFile(fsys fs.FS, name string, v any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
