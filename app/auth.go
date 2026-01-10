package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ------------------- Auth Helpers -------------------

func getAuthViewData(r *http.Request) AuthViewData {
	username, ok := getAuthenticatedUsername(r)
	return AuthViewData{
		IsAuthenticated: ok,
		Username:        username,
		CurrentPath:     r.URL.RequestURI(),
	}
}

func getAuthenticatedUsername(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return "", false
	}

	parts := strings.SplitN(cookie.Value, "|", 2)
	if len(parts) != 2 {
		return "", false
	}

	username := parts[0]
	signature := parts[1]
	if username == "" || signature == "" {
		return "", false
	}

	expected := signAuthCookie(username)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return "", false
	}

	if !userExists(db, username) {
		return "", false
	}

	return username, true
}

func signAuthCookie(username string) string {
	mac := hmac.New(sha256.New, auth.Secret)
	_, _ = mac.Write([]byte(username))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, username string) {
	value := fmt.Sprintf("%s|%s", username, signAuthCookie(username))
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   60 * 60 * 24 * 7,
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := getAuthenticatedUsername(r); ok {
		return true
	}

	next := r.URL.RequestURI()
	http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
	return false
}

func requireAPIAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := getBearerUsername(r); ok {
		return true
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

func getBearerUsername(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return verifyJWT(parts[1])
}

func issueJWT(username string, ttl time.Duration) (string, time.Time, error) {
	if username == "" {
		return "", time.Time{}, fmt.Errorf("missing username")
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	exp := time.Now().Add(ttl).Unix()
	payloadBytes, err := json.Marshal(map[string]interface{}{
		"sub": username,
		"exp": exp,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := header + "." + payload

	mac := hmac.New(sha256.New, auth.JWTSecret)
	_, _ = mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	token := unsigned + "." + signature
	return token, time.Unix(exp, 0), nil
}

func verifyJWT(token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	unsigned := parts[0] + "." + parts[1]

	mac := hmac.New(sha256.New, auth.JWTSecret)
	_, _ = mac.Write([]byte(unsigned))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return "", false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}

	var payload struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", false
	}
	if payload.Sub == "" {
		return "", false
	}
	if time.Now().Unix() > payload.Exp {
		return "", false
	}
	if !userExists(db, payload.Sub) {
		return "", false
	}
	return payload.Sub, true
}
