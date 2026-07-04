package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/taiiok/xiaomi-vless/internal/i18n"
)

type Auth struct {
	store interface {
		CheckPassword(username, password string) bool
	}
	tokens sync.Map
}

func NewAuth(store interface {
	CheckPassword(username, password string) bool
}) *Auth {
	return &Auth{store: store}
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.authenticated(r) {
			next.ServeHTTP(w, r)
			return
		}
		if isAPI(r) {
			locale := i18n.LocaleFromRequest(r)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": i18n.T(locale, "unauthorized")})
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func (a *Auth) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.ServeFileFS(w, r, staticFS, "login.html")
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if !a.store.CheckPassword(username, password) {
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}
	token := a.newToken()
	a.tokens.Store(token, true)
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *Auth) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		a.tokens.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *Auth) authenticated(r *http.Request) bool {
	if c, err := r.Cookie("session"); err == nil {
		if _, ok := a.tokens.Load(c.Value); ok {
			return true
		}
	}
	user, pass, ok := r.BasicAuth()
	if ok && a.store.CheckPassword(user, pass) {
		return true
	}
	return false
}

func (a *Auth) newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func isAPI(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
