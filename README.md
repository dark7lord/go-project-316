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

`--timeout` sets the timeout for each HTTP request, including reading its
response body (default `15s`). When a retry is made, that attempt gets the same
timeout limit; cancellation of the overall crawl can stop it sooner. When using
the library, set `crawler.Options.Timeout`; a shorter timeout already
configured on the supplied HTTP client takes precedence.
`--user-agent` overrides the HTTP client's default User-Agent header.
`--indent-json` controls JSON indentation (enabled by default; pass
`--indent-json=false` for compact JSON). The library's `IndentJSON` option is
`true` for indented output and `false` for compact output.

## JSON report

The library returns a JSON object with the crawl's `root_url`, configured
`depth`, UTC `generated_at` timestamp, and the list of `pages`. Each page
includes its URL and depth, HTTP status, crawl status, optional error, SEO
results, broken links, assets, and UTC `discovered_at` timestamp.

An `error` key is included only when a page or asset has an error. Empty
collections on successfully fetched pages are empty arrays; collections on
pages that could not be fetched are `null`. Timestamps use RFC3339, an ISO8601
format. For broken links, HTTP errors include the status text in `error`;
transport errors have a zero `status_code` and the transport error message.

```json
{
    "root_url": "https://example.com",
    "depth": 1,
    "generated_at": "2024-06-01T12:34:56Z",
    "pages": [
        {
            "url": "https://example.com",
            "depth": 0,
            "http_status": 200,
            "status": "ok",
            "seo": {
                "has_title": true,
                "title": "Example title",
                "has_description": true,
                "description": "Example description",
                "has_h1": true
            },
            "broken_links": [
                {
                    "url": "https://example.com/missing",
                    "status_code": 404,
                    "error": "Not Found"
                }
            ],
            "assets": [
                {
                    "url": "https://example.com/static/logo.png",
                    "type": "image",
                    "status_code": 200,
                    "size_bytes": 12345
                }
            ],
            "discovered_at": "2024-06-01T12:34:56Z"
        }
    ]
}
```

`status` is `ok` for successful pages and `error` when a page request fails or
returns an HTTP status of 400 or greater. SEO flags indicate whether a title,
description, or H1 element exists; the title and description fields contain
their extracted text.

The library's `IndentJSON` option and CLI's `--indent-json` flag control
indentation only. They do not change the report values or key order. The CLI
writes the JSON report without extra text and terminates it with a newline.

## Assets

Each successfully fetched page report includes an `assets` array for images,
external scripts, and stylesheets referenced by the page, sorted by type
(image, script, style). Asset entries contain `url`, `type`, `status_code`,
`size_bytes`, and `error` when an error occurred. The crawler uses
`Content-Length` when available and otherwise counts bytes while reading the
response body. Asset requests share the crawl's retries and request interval.

Duplicate asset URLs are removed within each page, and the fetched result is
shared across pages during the same crawl.

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
