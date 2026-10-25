package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const (
	adminCookie = "spyball_admin"
	// Long enough for a whole party, including the morning after.
	sessionMaxAge = 48 * time.Hour
)

// sessions stores small values in HMAC-signed cookies: the client can read them but not forge them.
// Admin and player sessions use different cookies, so the admin can also play from the same phone.
type sessions struct {
	key []byte
}

func (s sessions) set(w http.ResponseWriter, name, value string) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(value))
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    payload + "." + s.sign(name, payload),
		Path:     "/",
		MaxAge:   int(sessionMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s sessions) get(r *http.Request, name string) (string, bool) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", false
	}
	payload, sig, ok := strings.Cut(c.Value, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(s.sign(name, payload))) {
		return "", false
	}
	value, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", false
	}
	return string(value), true
}

func (s sessions) clear(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// sign binds the payload to the cookie name, so a player cookie can never be replayed as an admin cookie.
func (s sessions) sign(name, payload string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(name + "\x00" + payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
