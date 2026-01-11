package app

import (
	"database/sql"
	"embed"
	"html/template"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

var (
	db        *sql.DB
	templates *template.Template
	log       = logrus.New()
	auth      AuthConfig
	assetsFS  embed.FS
)

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)
}

func Run(dbPath string, templatesFS embed.FS) error {
	var err error

	db, err = openDatabase(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return err
	}

	if err := seedData(db); err != nil {
		log.Printf("Failed to seed data: %v", err)
	}

	assetsFS = templatesFS
	templates, err = parseTemplates(templatesFS)
	if err != nil {
		return err
	}

	auth = loadAuthConfig()

	if err := ensureSeedUser(db, auth.Username, auth.Password); err != nil {
		return err
	}

	r := buildRouter()
	log.Info("Server listening on http://localhost:8080")
	return http.ListenAndServe(":8080", r)
}
