.PHONY: help build run-server run-server-sqlite run-server-postgres-sql run-server-postgres-pgx run-server-stdout run-client run-pipeline run-pipeline-collector run-collector docker-up docker-down clean

help:
	@echo "Available commands:"
	@echo "  make build                     - Compile server, client, and pipeline binaries"
	@echo "  make run-collector             - Run standalone local OpenTelemetry Collector"
	@echo "  make run-server                - Run API server with default SQLite storage"
	@echo "  make run-server-sqlite         - Run API server with SQLite (otelsql)"
	@echo "  make run-server-postgres-sql   - Run API server with PostgreSQL (Approach A: otelsql)"
	@echo "  make run-server-postgres-pgx   - Run API server with PostgreSQL (Approach B: native otelpgx)"
	@echo "  make run-server-stdout         - Run API server in STDOUT mode (zero external dependencies)"
	@echo "  make run-client                - Run client simulator to send test requests"
	@echo "  make run-pipeline              - Run AI Multi-Stage Pipeline demo (Console STDOUT mode)"
	@echo "  make run-pipeline-collector    - Run AI Multi-Stage Pipeline demo with OTel Collector"
	@echo "  make docker-up                 - Run Postgres + Collector + Jaeger + Prometheus in Docker"
	@echo "  make docker-down               - Stop Docker Compose services"
	@echo "  make clean                     - Clean built binaries"

build:
	@mkdir -p bin
	go build -o bin/server ./cmd/server
	go build -o bin/client ./cmd/client
	go build -o bin/pipeline ./cmd/pipeline
	@echo "✓ Binaries built in bin/"

run-pipeline:
	go run ./cmd/pipeline -exporter=stdout

run-pipeline-collector:
	go run ./cmd/pipeline -exporter=otlp -collector-addr=localhost:4317

run-collector:
	./scripts/run-local-collector.sh

run-server: run-server-sqlite

run-server-sqlite:
	go run ./cmd/server -db=sqlite -db-dsn=orders.db -exporter=otlp -collector-addr=localhost:4317 -port=8080

run-server-postgres-sql:
	go run ./cmd/server -db=postgres-sql -db-dsn="postgres://postgres:postgrespassword@localhost:5433/ecommerce_db?sslmode=disable" -exporter=otlp -collector-addr=localhost:4317 -port=8080

run-server-postgres-pgx:
	go run ./cmd/server -db=postgres-pgx -db-dsn="postgres://postgres:postgrespassword@localhost:5433/ecommerce_db?sslmode=disable" -exporter=otlp -collector-addr=localhost:4317 -port=8080

run-server-stdout:
	go run ./cmd/server -db=sqlite -db-dsn=orders.db -exporter=stdout -port=8080

run-client:
	go run ./cmd/client -server=http://localhost:8080 -exporter=otlp -collector-addr=localhost:4317

docker-up:
	docker compose up -d
	@echo "✓ PostgreSQL available at:    localhost:5433 (user: postgres, pass: postgrespassword)"
	@echo "✓ Jaeger UI available at:     http://localhost:16686"
	@echo "✓ Prometheus UI available at: http://localhost:9090"
	@echo "✓ Collector OTLP gRPC at:     localhost:4317"
	@echo "✓ Collector OTLP HTTP at:     localhost:4318"

docker-down:
	docker compose down

clean:
	rm -rf bin/server bin/client *.db
