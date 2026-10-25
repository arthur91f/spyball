// Package server wires the HTTP routes, sessions and template rendering of Spyball.
package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"

	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/web"
)

const (
	layoutFile = "templates/layout.html"
	pagesGlob  = "templates/pages/*.html"
)

// Server is the Spyball HTTP handler.
type Server struct {
	catalog *config.Catalog
	mux     *http.ServeMux
	pages   map[string]*template.Template
}

// New parses the embedded templates, with interface texts taken from the given locale, and registers the routes.
func New(catalog *config.Catalog, lang string) (*Server, error) {
	locale, ok := catalog.Locales[lang]
	if !ok {
		return nil, fmt.Errorf("unknown locale %q", lang)
	}

	pages, err := parsePages(web.Templates, template.FuncMap{"t": locale.T})
	if err != nil {
		return nil, err
	}

	static, err := fs.Sub(web.Static, "static")
	if err != nil {
		return nil, err
	}

	s := &Server{catalog: catalog, mux: http.NewServeMux(), pages: pages}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	s.mux.HandleFunc("GET /{$}", s.handleHome)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
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
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}
