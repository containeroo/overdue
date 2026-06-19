package main

import (
	"context"
	"embed"
	"io/fs"
	"os"

	"github.com/containeroo/overdue/internal/app"
)

var (
	Version = "dev"
	Commit  = "none"
)

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

// main sets up the application context and runs the main loop.
func main() {
	ctx := context.Background()

	templateFS, err := fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		panic(err)
	}

	if err := app.Run(ctx, Version, Commit, os.Args[1:], os.Stdout, os.Stderr, templateFS); err != nil {
		os.Exit(1)
	}
}
