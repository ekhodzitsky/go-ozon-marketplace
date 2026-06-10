# go-ozon-marketplace

SOTA microservice e-commerce marketplace demo in Go. Built to demonstrate high-load patterns relevant to Ozon engineering.

## Architecture

8 microservices: API Gateway, User, Catalog, Order, Inventory, Payment, Notification, Analytics.

## Quick Start

```bash
make up
```

## Tech Stack

Go, gRPC, Kafka, PostgreSQL, Redis, ClickHouse, Elasticsearch, OpenTelemetry, Prometheus, Grafana, Kubernetes.

## Design Doc

See [docs/design.md](docs/design.md).

## Prerequisites

- Go 1.23+
- Docker & Docker Compose
- Buf (for proto generation)
- golangci-lint (for linting)

## TLS for gRPC

Generate CA + server/client certificates:

```bash
./scripts/generate-certs.sh certs
```

Enable TLS in a service by setting the `CERT_PATH` environment variable to the directory containing `server-cert.pem` and `server-key.pem` (e.g. `CERT_PATH=./certs`). If `CERT_PATH` is unset or empty, the gRPC server starts in plain-text mode.

The `pkg/server` package provides helpers for loading credentials:

- `server.LoadServerCredentials(certFile, keyFile)` — returns a `grpc.ServerOption`.
- `server.LoadClientCredentials(caFile, serverName)` — returns client `credentials.TransportCredentials`.
