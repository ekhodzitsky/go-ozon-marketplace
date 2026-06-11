# SLO / SLI

## Сервис: go-ozon-marketplace

| SLI | SLO | Метрика |
|-----|-----|---------|
| Availability | 99.9% | `up` metric |
| Latency p99 | < 500ms | `grpc_server_handling_seconds` |
| Error rate | < 1% | `grpc_server_handled_total` |
| Throughput | > 1000 RPS | `grpc_server_started_total` |
