.PHONY: help build run-server run-server-stdout run-client run-collector docker-up docker-down clean

help:
	@echo "Available commands:"
	@echo "  make build               - Compile server and client binaries"
	@echo "  make run-collector       - Run standalone local OpenTelemetry Collector"
	@echo "  make run-server          - Run API server connected to OTel Collector (OTLP gRPC)"
	@echo "  make run-server-stdout   - Run API server in STDOUT mode (zero external dependencies)"
	@echo "  make run-client          - Run client simulator to send test requests"
	@echo "  make docker-up           - Run Collector + Jaeger + Prometheus in Docker Compose"
	@echo "  make docker-down         - Stop Docker Compose services"
	@echo "  make clean               - Clean built binaries"

build:
	@mkdir -p bin
	go build -o bin/server ./cmd/server
	go build -o bin/client ./cmd/client
	@echo "✓ Binaries built in bin/"

run-collector:
	./scripts/run-local-collector.sh

run-server:
	go run ./cmd/server -exporter=otlp -collector-addr=localhost:4317 -port=8080

run-server-stdout:
	go run ./cmd/server -exporter=stdout -port=8080

run-client:
	go run ./cmd/client -server=http://localhost:8080 -exporter=otlp -collector-addr=localhost:4317

docker-up:
	docker compose up -d
	@echo "✓ Jaeger UI available at:     http://localhost:16686"
	@echo "✓ Prometheus UI available at: http://localhost:9090"
	@echo "✓ Collector OTLP gRPC at:     localhost:4317"
	@echo "✓ Collector OTLP HTTP at:     localhost:4318"

docker-down:
	docker compose down

clean:
	rm -rf bin/server bin/client
