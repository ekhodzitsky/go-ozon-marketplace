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
