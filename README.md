### Hexlet tests and linter status:
[![Actions Status](https://github.com/dark7lord/go-project-316/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/dark7lord/go-project-316/actions)
[![ci](https://github.com/dark7lord/go-project-316/actions/workflows/ci.yml/badge.svg)](https://github.com/dark7lord/go-project-316/actions/workflows/ci.yml)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=dark7lord_go-project-316&metric=coverage)](https://sonarcloud.io/summary/new_code?id=dark7lord_go-project-316)
<!-- [![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=dark7lord_go-project-316&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=dark7lord_go-project-316) -->

## Usage

```bash
make build                          # build binary into bin/hexlet-go-crawler
make test                           # run tests with coverage (coverage.out)
make run URL=https://example.com    # crawl a site and print JSON report
make lint                           # run golangci-lint
make check                          # run tests and linter
```

## Crawl depth

Depth is the number of link hops from the start URL:

- the start page has `depth: 0`;
- pages it links to have `depth: 1`, their links `depth: 2`, and so on.

The crawler visits pages while their depth is not greater than `--depth`
(default `10`). With `--depth 0` only the start page is checked, with
`--depth 1` the start page and the pages it links to.

Only pages on the same host as the start URL are crawled. Links to other
hosts are checked as possibly broken but never crawled. Each page appears
in the report once.

```bash
go run ./cmd/hexlet-go-crawler --depth 2 --workers 4 https://example.com
```

`--workers` sets the maximum number of pages processed concurrently (default
`4`). Values `0` and `1` run the crawl sequentially. The report is ordered by
depth and then URL.

## Assets

Each page report includes an `assets` array for images, external scripts, and
stylesheets referenced by the page. Asset entries contain `url`, `type`,
`status_code`, `size_bytes`, and `error`. The crawler uses `Content-Length`
when available and otherwise counts bytes while reading the response body.
Asset requests share the crawl's retries and request interval.

Duplicate asset URLs are removed within each page, and the fetched result is
shared across pages during the same crawl.

Example output:

```json
{
  "root_url": "https://example.com",
  "depth": 10,
  "generated_at": "2026-10-14T14:00:00Z",
  "pages": [
    {
      "url": "https://example.com",
      "depth": 0,
      "http_status": 200
    }
  ]
}
```

## Rate limiting

The first request starts immediately. With no rate limit configured, later
requests are spaced by at least `100ms`. Two flags can configure a longer
interval:

- `--delay` sets the minimal pause between the starts of two requests
  (example: `200ms`, `1s`);
- `--rps` limits the number of requests per second; it is converted into a
  delay of `1s / rps` (`--rps 5` is the same as `--delay 200ms`).

If both flags are set, `--rps` wins. A value of `0` means no limit beyond the
default `100ms` interval. The interval is global for the whole process: page
requests, link checks, and retries share it. Interrupting the crawler (Ctrl+C)
stops the wait immediately.

```bash
go run ./cmd/hexlet-go-crawler --rps 5 https://example.com
make run URL=https://example.com DELAY=200ms
```

## Retries

`--retries` sets the maximum number of additional attempts for a request
(default `1`). For example, `--retries 2` allows up to three requests total:
the initial request and two retries. Retrying stops when the response is not
retryable.

The crawler retries transport errors, except context cancellation and deadline
expiration, and HTTP responses that may be temporary: `408 Request Timeout`,
`429 Too Many Requests`, and status codes `500–599`. Other HTTP statuses, such
as `404 Not Found`, are returned without retrying.

Each retry observes the configured request interval; with no interval
configured, at least `100ms` passes before the next request.

```bash
go run ./cmd/hexlet-go-crawler --retries 2 https://example.com
```
