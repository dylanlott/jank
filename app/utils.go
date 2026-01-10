package app

import (
	"encoding/json"
	"net/http"
)

// respondJSON sends JSON responses (for our REST endpoints).
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}
