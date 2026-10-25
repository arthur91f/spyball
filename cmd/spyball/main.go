// Command spyball runs the Spyball party game web server.
package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arthur91f/spyball"
	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	contentDir := flag.String("content", "", "directory with themes/, setups/ and locales/ (default: content built into the binary)")
	lang := flag.String("lang", "fr", "interface language, a file of locales/")
	flag.Parse()

	if err := run(*addr, *contentDir, *lang); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(addr, contentDir, lang string) error {
	var content fs.FS = spyball.Content
	if contentDir != "" {
		content = os.DirFS(contentDir)
	}
	catalog, err := config.Load(content)
	if err != nil {
		return err
	}
	slog.Info("content loaded", "themes", len(catalog.Themes), "setups", len(catalog.Setups), "locales", len(catalog.Locales))

	srv, err := server.New(catalog, lang)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server started", "addr", addr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
