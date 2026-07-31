package main

import (
	"github.com/bethropolis/dir-dumper/internal/app"
	"github.com/bethropolis/dir-dumper/internal/config"
)

func main() {
	// Load configuration from command-line flags
	cfg := config.New()

	// Create and run the application
	application := app.New(cfg)

	// Run the application
	application.Run()

	// Clean up
	application.Close()
}
