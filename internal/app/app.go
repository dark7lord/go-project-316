// Package app is the command line interface of hexlet-go-crawler.
package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"code/crawler"
)

const version = "1.0.0"

func flags() []cli.Flag {
	return []cli.Flag{
		&cli.UintFlag{Name: "depth", Value: 10, Usage: "crawl depth"},
		&cli.UintFlag{Name: "retries", Value: 1, Usage: "number of retries for failed requests"},
		&cli.DurationFlag{Name: "delay", Value: 0, Usage: "delay between requests (example: 200ms, 1s)"},
		&cli.DurationFlag{Name: "timeout", Value: 15 * time.Second, Usage: "per-request timeout"},
		&cli.UintFlag{Name: "rps", Value: 0, Usage: "limit requests per second (overrides delay)"},
		&cli.StringFlag{Name: "user-agent", Usage: "custom user agent"},
		&cli.UintFlag{Name: "workers", Value: 4, Usage: "number of concurrent workers"},
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() == 0 || cmd.Args().Get(0) == "" {
		return cli.ShowRootCommandHelp(cmd)
	}

	if cmd.NArg() > 1 {
		args := strings.Join(cmd.Args().Slice(), " ")

		return fmt.Errorf("expected 1 url path, got '%s'", args)
	}

	opts := crawler.Options{
		URL:     cmd.Args().Get(0),
		Depth:   cmd.Uint("depth"),
		Retries: cmd.Uint("retries"),
		Delay:   cmd.Duration("delay"),
		Timeout: cmd.Duration("timeout"),
		// TODO: RPS?
		UserAgent:   cmd.String("user-agent"),
		Concurrency: cmd.Uint("workers"),
		HTTPClient:  &http.Client{Timeout: cmd.Duration("timeout")},
		IndentJSON:  4, // TODO: think about getting from flags
	}

	data, errAnalyze := crawler.Analyze(ctx, opts)
	if errAnalyze != nil {
		return fmt.Errorf("analyze error: %w", errAnalyze)
	}

	_, err := fmt.Fprintln(cmd.Root().Writer, string(data))

	return err
}

func newCommand(stdout, stderr io.Writer) *cli.Command {
	return &cli.Command{
		Name:        "hexlet-go-crawler",
		Description: "analyze a website structure",
		Usage:       "analyze a website structure",
		ArgsUsage:   "<url>",
		Writer:      stdout,
		ErrWriter:   stderr,
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "print version",
				Action: func(_ context.Context, cmd *cli.Command) error {
					_, err := fmt.Fprintln(cmd.Root().Writer, version)

					return err
				},
			},
		},
		Flags:  flags(),
		Action: run,
	}
}

// Run executes the CLI with args and returns the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if err := newCommand(stdout, stderr).Run(ctx, args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)

		return 1
	}

	return 0
}
