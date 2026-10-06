BIN := bin/hexlet-go-crawler

build:
	go build -o $(BIN) cmd/hexlet-go-crawler/main.go

test:
	go test -coverprofile=coverage.out ./...

run:
	go run cmd/hexlet-go-crawler/main.go "$(URL)"

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

fmt:
	golangci-lint fmt

.PHONY: build test run lint lint-fix fmt
