package app

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

var (
	db        *sql.DB
	dbDriver  string
	templates *template.Template
	log       = logrus.New()
	auth      AuthConfig
	assetsFS  embed.FS
)

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)
}

func Run(templatesFS embed.FS) error {
	var err error

	db, err = openDatabase()
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

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-shutdownCtx.Done():
		log.Info("Shutdown signal received")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}

	err = <-serverErr
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
