# ChronosDB Makefile
# Single-node bitemporal graph database with learned shortcuts.

.PHONY: build test test-race lint vet bench clean \
        build-chronosd build-importer build-webserver \
        import-csv import-json import-sql import-all \
        query-sample docker-build docker-up docker-down

# ---- Build ----

build: build-chronosd build-importer build-webserver

build-chronosd:
	go build -o bin/chronosd ./cmd/chronosd

build-importer:
	go build -o bin/importer ./cmd/importer

build-webserver:
	go build -o bin/webserver ./cmd/webserver

# ---- Quality gates (guardrail: all must pass before merge) ----

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

lint:
	@command -v staticcheck >/dev/null || { echo "staticcheck not installed: run 'go install honnef.co/go/tools/cmd/staticcheck@latest'"; exit 1; }
	staticcheck ./...

bench:
	./scripts/bench.sh

# ---- Data ingestion (sample fixtures) ----

import-csv: build-importer
	./bin/importer -file test/data/sample.csv -format csv -label Customer -timestamp timestamp

import-json: build-importer
	./bin/importer -file test/data/sample.json -format json -label Person

import-sql: build-importer
	./bin/importer -file test/data/sample.sql -format sql -label User

import-all: import-csv import-json import-sql

query-sample:
	@curl -X POST http://localhost:8080/v1/db/test/query \
	  -H "Content-Type: application/json" \
	  -d '{"query": "MATCH (n:Customer) RETURN n"}'

# ---- Docker ----

docker-build:
	docker build -t chronosdb:dev .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# ---- Cleanup ----

clean:
	rm -rf bin/ data/ bench-results.json
