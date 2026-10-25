package server

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arthur91f/spyball"
	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	catalog, err := config.Load(spyball.Content)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "game.json"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	srv, err := New(catalog, st, "fr")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func TestRoutes(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name        string
		path        string
		wantStatus  int
		wantType    string
		wantContain string
	}{
		{"home", "/", http.StatusOK, "text/html", "Trouvez vos alliés"},
		{"stylesheet", "/static/style.css", http.StatusOK, "text/css", "--team-blue"},
		{"unknown page", "/nope", http.StatusNotFound, "", ""},
		{"unknown asset", "/static/nope.css", http.StatusNotFound, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, tt.wantType) {
				t.Errorf("Content-Type = %q, want prefix %q", got, tt.wantType)
			}
			body, _ := io.ReadAll(rec.Body)
			if !strings.Contains(string(body), tt.wantContain) {
				t.Errorf("body does not contain %q:\n%s", tt.wantContain, body)
			}
		})
	}
}

func TestHomeUsesLayout(t *testing.T) {
	srv := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()

	for _, want := range []string{`<html lang="fr">`, `name="viewport"`, `href="/static/style.css"`} {
		if !strings.Contains(body, want) {
			t.Errorf("home page missing %q", want)
		}
	}
}

func TestMissingTranslationFailsRender(t *testing.T) {
	catalog, err := config.Load(spyball.Content)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	fsys := fstest.MapFS{
		"templates/layout.html":       {Data: []byte(`{{define "layout"}}{{block "content" .}}{{end}}{{end}}`)},
		"templates/pages/broken.html": {Data: []byte(`{{define "content"}}{{t "no.such.key"}}{{end}}`)},
	}
	pages, err := parsePages(fsys, template.FuncMap{"t": catalog.Locales["fr"].T})
	if err != nil {
		t.Fatalf("parsePages: %v", err)
	}

	rec := httptest.NewRecorder()
	(&Server{pages: pages}).render(rec, http.StatusOK, "broken", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "no.such.key") {
		t.Error("partial page leaked to the client")
	}
}
