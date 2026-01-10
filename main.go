package main

import (
	"embed"
	"log"

	"jank/app"
)

//go:embed templates/*.html
var templatesFS embed.FS

func main() {
	if err := app.Run("./sqlite.db", templatesFS); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
