BIN := bin/hexlet-go-crawler

build:
	go build -o $(BIN) cmd/hexlet-go-crawler/main.go

test:
	go test -coverprofile=coverage.out ./...

URL ?= localhost:7070
DEPTH ?= 3

run:
	go run cmd/hexlet-go-crawler/main.go -depth $(DEPTH) "$(URL)" 

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

fmt:
	golangci-lint fmt

cover:
	go tool cover -func=coverage.out

cover-html:
	go tool cover -html=coverage.out

check: test lint

clean:
	rm -f $(BIN) coverage*.out

dev-server:
	python3 -m http.server 7070 -d crawler/testdata

.PHONY: build test run lint lint-fix fmt check clean cover cover-html dev-server
