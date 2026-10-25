// Command spyball runs the Spyball party game web server.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/arthur91f/spyball"
	"github.com/arthur91f/spyball/internal/config"
	"github.com/arthur91f/spyball/internal/server"
	"github.com/arthur91f/spyball/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	contentDir := flag.String("content", "", "directory with themes/, setups/ and locales/ (default: content built into the binary)")
	lang := flag.String("lang", "fr", "interface language, a file of locales/")
	dataDir := flag.String("data", "data", "directory where the game state and the session key are saved")
	adminPassword := flag.String("admin-password", os.Getenv("SPYBALL_ADMIN_PASSWORD"),
		"admin password (default: $SPYBALL_ADMIN_PASSWORD, or a random one printed at startup)")
	flag.Parse()

	if err := run(*addr, *contentDir, *lang, *dataDir, *adminPassword); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(addr, contentDir, lang, dataDir, adminPassword string) error {
	var content fs.FS = spyball.Content
	if contentDir != "" {
		content = os.DirFS(contentDir)
	}
	catalog, err := config.Load(content)
	if err != nil {
		return err
	}
	slog.Info("content loaded", "themes", len(catalog.Themes), "setups", len(catalog.Setups), "locales", len(catalog.Locales))

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(dataDir, "game.json"))
	if err != nil {
		return err
	}
	slog.Info("game state", "file", st.Path())

	sessionKey, err := loadOrCreateKey(filepath.Join(dataDir, "session.key"))
	if err != nil {
		return err
	}
	if adminPassword == "" {
		adminPassword = rand.Text()[:10]
		slog.Warn("no admin password given, using a random one", "admin_password", adminPassword)
	}

	srv, err := server.New(catalog, st, server.Options{Lang: lang, AdminPassword: adminPassword, SessionKey: sessionKey})
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

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	slog.Info("server started", "addr", ln.Addr().String())
	for _, url := range shareURLs(ln.Addr().(*net.TCPAddr)) {
		slog.Info("players can join at", "url", url)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(ln)
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

// loadOrCreateKey returns the secret key stored at path, creating it on first run,
// so player and admin sessions survive a server restart.
func loadOrCreateKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil && len(key) >= 32 {
		return key, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, os.WriteFile(path, key, 0o600)
}

// shareURLs lists the addresses guests can type on their phone: one per local network address
// when listening on all interfaces, else the listening address itself.
func shareURLs(listen *net.TCPAddr) []string {
	port := strconv.Itoa(listen.Port)
	if !listen.IP.IsUnspecified() {
		return []string{"http://" + net.JoinHostPort(listen.IP.String(), port)}
	}

	var urls []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil || ipnet.IP.IsLinkLocalUnicast() {
			continue
		}
		urls = append(urls, "http://"+net.JoinHostPort(ipnet.IP.String(), port))
	}
	if len(urls) == 0 {
		urls = append(urls, "http://localhost:"+port)
	}
	return urls
}
