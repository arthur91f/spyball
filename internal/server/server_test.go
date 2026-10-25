package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	srv, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantStatus  int
		wantType    string
		wantContain string
	}{
		{"home", "/", http.StatusOK, "text/html", "<h1>Spyball</h1>"},
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
	srv, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()

	for _, want := range []string{`<html lang="fr">`, `name="viewport"`, `href="/static/style.css"`} {
		if !strings.Contains(body, want) {
			t.Errorf("home page missing %q", want)
		}
	}
}
