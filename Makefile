.PHONY: up down build test lint loadtest

up:            ## start local dependencies (Postgres, Redis, Redpanda, LocalStack)
	docker compose up -d

down:          ## stop and remove local dependencies
	docker compose down

build:         ## build all binaries into ./bin
	go build -o bin/ingest    ./cmd/ingest
	go build -o bin/consumer  ./cmd/consumer
	go build -o bin/commander ./cmd/commander
	go build -o bin/agent     ./cmd/agent

test:
	go test ./... -race -cover

lint:
	go vet ./...

loadtest:      ## requires k6 (see docs/DAY1_SETUP.md)
	k6 run loadtest/ingest.js
