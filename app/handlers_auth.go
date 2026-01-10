package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ------------------- Auth Handlers -------------------

func authTokenHandler(w http.ResponseWriter, r *http.Request) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if !authenticateUser(db, credentials.Username, credentials.Password) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token, expiresAt, err := issueJWT(credentials.Username, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to issue token", http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]interface{}{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func authSignupHandler(w http.ResponseWriter, r *http.Request) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	credentials.Username = strings.TrimSpace(credentials.Username)
	if credentials.Username == "" || credentials.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}
	if _, err := createUser(db, credentials.Username, credentials.Password); err != nil {
		log.Errorf("Failed to create user: %v", err)
		http.Error(w, signupErrorMessage(err), http.StatusBadRequest)
		return
	}
	token, expiresAt, err := issueJWT(credentials.Username, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to issue token", http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]interface{}{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}
