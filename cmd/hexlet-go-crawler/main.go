// Package main is the entry point of the hexlet-go-crawler CLI.
package main

import (
	"context"
	"os"

	"code/internal/app"
)

func main() {
	ctx := context.Background()
	codeExit := app.Run(ctx, os.Args, os.Stdout, os.Stderr)
	os.Exit(codeExit)
}
