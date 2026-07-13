.PHONY: deps test coverage coverage-html coverage-check coverage-badge lint build-server build-client build-client-all run-server clean

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)

deps:
	go mod download
	go mod tidy

build-server:
	mkdir -p bin
	go build -o bin/gophkeeper-server ./cmd/server

build-client:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper ./cmd/client

build-client-all:
	mkdir -p bin
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-linux-amd64 ./cmd/client
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-darwin-arm64 ./cmd/client
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-darwin-amd64 ./cmd/client
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-windows-amd64.exe ./cmd/client

run-server: build-server
	./bin/gophkeeper-server -d "$${DATABASE_URI:?set DATABASE_URI}"

test:
	go test -count=1 -p 1 ./...

coverage:
	@go test -p 1 -coverprofile=coverage.out $$(go list ./... | grep -v '/cmd/')
	@go tool cover -func=coverage.out | tail -1

coverage-html: coverage
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

coverage-check: coverage
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $$COVERAGE%"; \
	awk -v c="$$COVERAGE" 'BEGIN { exit !(c+0 >= 75) }' || (echo "Coverage below 75%"; exit 1)

coverage-badge: coverage
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	COLOR="red"; \
	if [ $$(echo "$$COVERAGE > 70" | bc -l) -eq 1 ]; then \
		COLOR="brightgreen"; \
	elif [ $$(echo "$$COVERAGE >= 60" | bc -l) -eq 1 ]; then \
		COLOR="yellow"; \
	fi; \
	mkdir -p .github/badges; \
	echo "{\"schemaVersion\": 1, \"label\": \"coverage\", \"message\": \"$${COVERAGE}%\", \"color\": \"$$COLOR\"}" > .github/badges/coverage.json; \
	echo "Coverage badge updated: $$COVERAGE% ($$COLOR)"

lint:
	golangci-lint run ./...

clean:
	rm -rf bin coverage.out coverage.html
