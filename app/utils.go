package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	maxThreadTags  = 6
	maxTagLength   = 24
)

var (
	errTagCount  = errors.New("tag count exceeds limit")
	errTagLength = errors.New("tag length exceeds limit")
)

// respondJSON sends JSON responses (for our REST endpoints).
func respondJSON(w http.ResponseWriter, data interface{}) {
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Errorf("Failed to encode JSON response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(append(payload, '\n')); err != nil {
		log.Errorf("Failed to write JSON response: %v", err)
	}
}

func validateTags(tags []string) ([]string, error) {
	normalized := normalizeTags(tags)
	if len(normalized) > maxThreadTags {
		return nil, errTagCount
	}
	for _, tag := range normalized {
		if utf8.RuneCountInString(tag) > maxTagLength {
			return nil, errTagLength
		}
	}
	return normalized, nil
}

func parseTagsInput(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t'
	})
	return normalizeTags(parts)
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{})
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		value := strings.TrimSpace(tag)
		value = strings.TrimPrefix(value, "#")
		value = strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func tagsFromString(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	return normalizeTags(parts)
}

func makeExcerpt(content string, limit int) string {
	if limit <= 0 {
		return ""
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	compact := strings.Join(strings.Fields(trimmed), " ")
	if utf8.RuneCountInString(compact) <= limit {
		return compact
	}
	if limit <= 3 {
		return string([]rune(compact)[:limit])
	}
	return string([]rune(compact)[:limit-3]) + "..."
}
