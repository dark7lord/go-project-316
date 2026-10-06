// Package main is the entry point of the hexlet-go-crawler CLI.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"code/crawler"
)

const version = "1.0.0"

func main() {
	if err := newCommand(os.Stdout).Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newCommand(out io.Writer) *cli.Command {
	return &cli.Command{
		Name:        "hexlet-go-crawler",
		Description: "analyze a website structure",
		Usage:       "analyze a website structure",
		ArgsUsage:   "<url>",
		Writer:      out,
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "print version",
				Action: func(context.Context, *cli.Command) error {
					_, err := fmt.Fprintln(out, version)

					return err
				},
			},
		},
		Flags:  flags(),
		Action: run,
	}
}

func flags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{Name: "depth", Value: 10, Usage: "crawl depth"},
		&cli.IntFlag{Name: "retries", Value: 1, Usage: "number of retries for failed requests"},
		&cli.StringFlag{Name: "delay", Value: "0s", Usage: "delay between requests (example: 200ms, 1s)"},
		&cli.StringFlag{Name: "timeout", Value: "15s", Usage: "per-request timeout"},
		&cli.IntFlag{Name: "rps", Value: 0, Usage: "limit requests per second (overrides delay)"},
		&cli.StringFlag{Name: "user-agent", Usage: "custom user agent"},
		&cli.IntFlag{Name: "workers", Value: 4, Usage: "number of concurrent workers"},
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return cli.ShowRootCommandHelp(cmd)
	}

	if cmd.NArg() > 1 {
		args := strings.Join(cmd.Args().Slice(), " ")

		return fmt.Errorf("expected 1 url path, got '%s'", args)
	}

	result := crawler.Analyze(ctx, crawler.Options{})

	_, err := fmt.Fprintf(cmd.Root().Writer, "%s\n%s\n", cmd.Args().Get(0), result)

	return err
}
