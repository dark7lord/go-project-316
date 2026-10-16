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
