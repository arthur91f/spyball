// Package server wires the HTTP routes, sessions and template rendering of Spyball.
package server

import (
	"bytes"
	crand "crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"path"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/store"
	"github.com/arthur91f/spyball/web"
)

const (
	layoutFile = "templates/layout.html"
	pagesGlob  = "templates/pages/*.html"
)

// Options configures a Server.
type Options struct {
	Lang          string // a locale of the catalog
	AdminPassword string
	SessionKey    []byte // signs session cookies; keep it stable so sessions survive a restart
	// NewRand returns the random source of a draw. Defaults to a cryptographically seeded one.
	NewRand func() *rand.Rand
}

// Server is the Spyball HTTP handler.
type Server struct {
	catalog  *config.Catalog
	store    *store.Store
	locale   *config.Locale
	opts     Options
	sessions sessions
	handler  http.Handler
	pages    map[string]*template.Template
}

// New parses the embedded templates and registers the routes. The game is read from and saved to st.
func New(catalog *config.Catalog, st *store.Store, opts Options) (*Server, error) {
	locale, ok := catalog.Locales[opts.Lang]
	if !ok {
		return nil, fmt.Errorf("unknown locale %q", opts.Lang)
	}
	if opts.AdminPassword == "" {
		return nil, errors.New("empty admin password")
	}
	if len(opts.SessionKey) < 32 {
		return nil, errors.New("session key must be at least 32 bytes")
	}
	if opts.NewRand == nil {
		opts.NewRand = func() *rand.Rand {
			var seed [32]byte
			_, _ = crand.Read(seed[:])
			return rand.New(rand.NewChaCha8(seed))
		}
	}

	pages, err := parsePages(web.Templates, template.FuncMap{
		"t": locale.T,
		"tf": func(key string, args ...any) (string, error) {
			msg, err := locale.T(key)
			return fmt.Sprintf(msg, args...), err
		},
	})
	if err != nil {
		return nil, err
	}

	static, err := fs.Sub(web.Static, "static")
	if err != nil {
		return nil, err
	}

	s := &Server{
		catalog:  catalog,
		store:    st,
		locale:   locale,
		opts:     opts,
		sessions: sessions{key: opts.SessionKey},
		pages:    pages,
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /{$}", s.handleHome)
	s.registerAdmin(mux)
	s.registerPlayer(mux)

	// Rejects cross-site form submissions (CSRF) based on the browser's Origin / Sec-Fetch-Site headers.
	s.handler = http.NewCrossOriginProtection().Handler(mux)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "home", nil)
}

// parsePages builds one template set per page, each combining the shared layout
// with the page file, so pages can all define the same "title" and "content" blocks.
func parsePages(fsys fs.FS, funcs template.FuncMap) (map[string]*template.Template, error) {
	files, err := fs.Glob(fsys, pagesGlob)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no page templates matching %s", pagesGlob)
	}

	pages := make(map[string]*template.Template, len(files))
	for _, file := range files {
		tmpl, err := template.New(path.Base(layoutFile)).Funcs(funcs).ParseFS(fsys, layoutFile, file)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}
		name := path.Base(file)
		pages[name[:len(name)-len(path.Ext(name))]] = tmpl
	}
	return pages, nil
}

// render executes a page into a buffer first, so a template error never sends a half-written page.
func (s *Server) render(w http.ResponseWriter, status int, page string, data any) {
	tmpl, ok := s.pages[page]
	if !ok {
		slog.Error("unknown page template", "page", page)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		slog.Error("render page", "page", page, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store") // pages show secrets: never keep them in the browser cache
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}
