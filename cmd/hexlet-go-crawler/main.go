// Package main is the entry point of the hexlet-go-crawler CLI.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"code/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	code := app.Run(ctx, os.Args, os.Stdout, os.Stderr)

	stop()
	os.Exit(code)
}
