# Makefile for ChronosDB import commands

.PHONY: build-importer import-csv import-json import-sql import-all query-csv query-json query-sql

build-importer:
    go build -o bin/importer ./cmd/importer

import-csv: build-importer
    ./bin/importer -file test/data/sample.csv -format csv -label Customer -timestamp timestamp

import-json: build-importer
    ./bin/importer -file test/data/sample.json -format json -label Person

import-sql: build-importer
    ./bin/importer -file test/data/sample.sql -format sql -label User

import-large: build-importer
    ./bin/importer -file test/data/large_sample.csv -format csv -label Customer -timestamp timestamp

import-all: import-csv import-json import-sql

query-csv:
    @curl -X POST http://localhost:8080/v1/db/test/query \
      -H "Content-Type: application/json" \
      -d '{"query": "MATCH (n:Customer) RETURN n"}'

query-stats:
    @curl -X POST http://localhost:8080/v1/db/test/query \
      -H "Content-Type: application/json" \
      -d '{"query": "MATCH (n:Customer) RETURN count(n) as total, avg(n.age) as avg_age, sum(n.amount) as total_amount"}'

query-forecast:
    @curl -X POST http://localhost:8080/v1/db/test/query \
      -H "Content-Type: application/json" \
      -d '{"query": "FORECAST amount OVER 30 DAYS FOR Customer_1"}'
