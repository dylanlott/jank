package main

import (
	"embed"
	"log"

	"jank/app"
)

//go:embed templates/*.html static/*
var contentFS embed.FS

func main() {
	if err := app.Run("./sqlite.db", contentFS); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
