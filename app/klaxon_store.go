package app

import (
	"database/sql"
	"strings"
	"time"
)

func getKlaxon(db *sql.DB) (*Klaxon, error) {
	row := db.QueryRow(`SELECT id, tone, emoji, message, updated_at FROM klaxons WHERE id = 1`)
	var klaxon Klaxon
	if err := row.Scan(&klaxon.ID, &klaxon.Tone, &klaxon.Emoji, &klaxon.Message, &klaxon.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &klaxon, nil
}

func saveKlaxon(db *sql.DB, tone, emoji, message string, updatedAt time.Time) error {
	tone = normalizeKlaxonTone(tone)
	emoji = strings.TrimSpace(emoji)
	message = strings.TrimSpace(message)

	if message == "" {
		_, err := db.Exec(`DELETE FROM klaxons WHERE id = 1`)
		return err
	}

	_, err := db.Exec(
		`INSERT INTO klaxons (id, tone, emoji, message, updated_at)
		VALUES (1, $1, $2, $3, $4)
		ON CONFLICT(id) DO UPDATE SET
			tone = excluded.tone,
			emoji = excluded.emoji,
			message = excluded.message,
			updated_at = excluded.updated_at`,
		tone, emoji, message, updatedAt,
	)
	return err
}

func normalizeKlaxonTone(tone string) string {
	tone = strings.ToLower(strings.TrimSpace(tone))
	switch tone {
	case "warn":
		return "warning"
	case "info", "warning", "danger", "success":
		return tone
	default:
		return "info"
	}
}
