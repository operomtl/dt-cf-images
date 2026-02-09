.PHONY: build test lint docker-build docker-up docker-down clean

build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

test:
	go test -race -count=1 ./internal/...

test-e2e:
	go test -race -count=1 -tags=e2e ./test/e2e/...

test-conformance:
	go test -race -count=1 -tags=conformance ./test/conformance/...

test-all: test test-e2e test-conformance

lint:
	golangci-lint run ./...

coverage:
	go test -race -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o cover.html

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-test: docker-build docker-up
	@echo "Waiting for service to be healthy..."
	@sleep 3
	@curl -sf http://localhost:8080/health && echo " OK" || (echo " FAILED"; docker compose down; exit 1)
	@docker compose down

clean:
	rm -rf bin/ coverage.out cover.html
