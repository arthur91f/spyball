package server

import (
	"io"
	"io/fs"
	"math/rand/v2"
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

const testAdminPassword = "s3cret"

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return newTestServerAt(t, t.TempDir())
}

// traitorsSetup is a test-only variant of 12-classic with two traitors per team.
const traitorsSetup = `
name: Traîtres à 12
weight: 1
challenges: 2
slots:
  blue-1: { traitor: true, clues: [blue-2, red-1] }
  blue-2: { clues: [blue-3, red-2] }
  blue-3: { traitor: true, clues: [blue-4, red-3] }
  blue-4: { clues: [blue-5, red-4] }
  blue-5: { clues: [blue-6, red-5] }
  blue-6: { clues: [blue-1, red-6] }
  red-1: { traitor: true, clues: [red-2, blue-2] }
  red-2: { clues: [red-3, blue-3] }
  red-3: { clues: [red-4, blue-4] }
  red-4: { traitor: true, clues: [red-5, blue-5] }
  red-5: { clues: [red-6, blue-6] }
  red-6: { clues: [red-1, blue-1] }
`

// testContent is the embedded content plus the setups/12-traitors.yaml test setup.
func testContent(t *testing.T) fs.FS {
	t.Helper()
	files := fstest.MapFS{"setups/12-traitors.yaml": {Data: []byte(traitorsSetup)}}
	err := fs.WalkDir(spyball.Content, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(spyball.Content, path)
		files[path] = &fstest.MapFile{Data: data}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// newTestServerAt starts a server saving its game in dataDir; two servers on the same dir model a restart.
func newTestServerAt(t *testing.T, dataDir string) *Server {
	t.Helper()
	catalog, err := config.Load(testContent(t))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	st, err := store.Open(filepath.Join(dataDir, "game.json"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	srv, err := New(catalog, st, Options{
		Lang:          "fr",
		AdminPassword: testAdminPassword,
		SessionKey:    []byte("0123456789abcdef0123456789abcdef"),
		NewRand:       func() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) },
	})
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
		{"unknown page", "/nope", http.StatusNotFound, "text/html", "Cette page n&#39;existe pas."},
		{"favicon", "/static/favicon.svg", http.StatusOK, "image/svg+xml", "<svg"},
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
	pages, err := parsePages(fsys, textFuncs(catalog.Locales["fr"], nil))
	if err != nil {
		t.Fatalf("parsePages: %v", err)
	}
	srv := newTestServer(t)
	srv.pages = pages

	rec := httptest.NewRecorder()
	srv.render(rec, http.StatusOK, "broken", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "no.such.key") {
		t.Error("partial page leaked to the client")
	}
}
