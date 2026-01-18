package app

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"os"
	"strings"
)

// AuthConfig holds credentials and signing secret for auth cookies.
type AuthConfig struct {
	Username  string
	Password  string
	Secret    []byte
	JWTSecret []byte
}

const authCookieName = "jank_auth"

func openDatabase() (*sql.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("JANK_DB_DRIVER")))
	dsn := strings.TrimSpace(os.Getenv("JANK_DB_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}

	if driver == "" {
		driver = "postgres"
	}

	switch driver {
	case "postgres", "postgresql", "pgx":
		driver = "pgx"
		if dsn == "" {
			return nil, fmt.Errorf("postgres selected; set JANK_DB_DSN or DATABASE_URL")
		}
	case "sqlite", "sqlite3":
		driver = "sqlite3"
		if dsn == "" {
			dsn = "./sqlite.db"
			log.Warn("JANK_DB_DSN not set; defaulting to ./sqlite.db")
		}
	default:
		return nil, fmt.Errorf("unsupported JANK_DB_DRIVER %q", driver)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	if driver == "sqlite3" {
		_, _ = db.Exec("PRAGMA foreign_keys = ON")
	}

	dbDriver = driver
	return db, nil
}

func parseTemplates(fs embed.FS) (*template.Template, error) {
	funcs := template.FuncMap{
		"markdown": renderMarkdown,
	}
	return template.New("base").Funcs(funcs).ParseFS(fs, "templates/*.html")
}

// ------------------- Auth Config -------------------

func loadAuthConfig() AuthConfig {
	username := strings.TrimSpace(os.Getenv("JANK_FORUM_USER"))
	password := strings.TrimSpace(os.Getenv("JANK_FORUM_PASS"))
	secret := strings.TrimSpace(os.Getenv("JANK_FORUM_SECRET"))
	jwtSecret := strings.TrimSpace(os.Getenv("JANK_JWT_SECRET"))

	if username == "" {
		username = "admin"
		log.Warn("JANK_FORUM_USER not set; defaulting to 'admin'")
	}
	if password == "" {
		password = "admin"
		log.Warn("JANK_FORUM_PASS not set; defaulting to 'admin'")
	}
	if secret == "" {
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			log.Fatalf("Failed to generate auth secret: %v", err)
		}
		log.Warn("JANK_FORUM_SECRET not set; using a random secret for this process")
		config := AuthConfig{
			Username: username,
			Password: password,
			Secret:   secretBytes,
		}
		if jwtSecret == "" {
			jwtBytes := make([]byte, 32)
			if _, err := rand.Read(jwtBytes); err != nil {
				log.Fatalf("Failed to generate JWT secret: %v", err)
			}
			log.Warn("JANK_JWT_SECRET not set; using a random JWT secret for this process")
			config.JWTSecret = jwtBytes
		} else {
			config.JWTSecret = []byte(jwtSecret)
		}
		return config
	}

	if jwtSecret == "" {
		log.Warn("JANK_JWT_SECRET not set; defaulting to JANK_FORUM_SECRET")
		jwtSecret = secret
	}

	return AuthConfig{
		Username:  username,
		Password:  password,
		Secret:    []byte(secret),
		JWTSecret: []byte(jwtSecret),
	}
}
