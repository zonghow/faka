package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const CookieName = "tikawang_session"

type SessionPayload struct {
	Authenticated bool  `json:"authenticated"`
	Exp           int64 `json:"exp"`
}

type Manager struct {
	secret []byte
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) CreateCookie(authenticated bool, maxAge time.Duration) (*http.Cookie, error) {
	payload := SessionPayload{
		Authenticated: authenticated,
		Exp:           time.Now().Add(maxAge).Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	sig := m.sign(raw)
	value := base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(sig)
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(maxAge.Seconds()),
	}, nil
}

func (m *Manager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}

func (m *Manager) Parse(r *http.Request) bool {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	if !hmac.Equal(sig, m.sign(raw)) {
		return false
	}
	var payload SessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	if !payload.Authenticated || time.Now().Unix() > payload.Exp {
		return false
	}
	return true
}

func (m *Manager) sign(raw []byte) []byte {
	h := hmac.New(sha256.New, m.secret)
	_, _ = h.Write(raw)
	return h.Sum(nil)
}
