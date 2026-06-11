# Развёртывание

Как запустить маркетплейс локально и в Kubernetes.

## Локальная разработка

### Через Docker Compose

```bash
# Инфраструктура (базы, брокеры, мониторинг)
make up

# Проверить статус
docker compose -f infra/docker/docker-compose.yml ps

# Логи инфраструктуры
docker compose -f infra/docker/docker-compose.yml logs -f postgres

# Остановить и удалить volumes
make down
```

> **Важно:** `docker-compose.yml` содержит только инфраструктуру (базы, брокеры, мониторинг). Сами сервисы запускаются отдельно через `go run` или Docker build.

### Вручную (Go)

```bash
# 1. Инфраструктура
make up

# 2. Каждый сервис в отдельном терминале (обязательны env vars)
export POSTGRES_DSN="postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"
export JWT_SECRET="min-32-chars-secret-key-here!!!"

cd services/<service> && go run ./cmd/...
```

### Переменные окружения

| Переменная | Описание | Пример |
|------------|----------|--------|
| `GRPC_PORT` | Порт gRPC сервера | `50055` |
| `POSTGRES_DSN` | Строка подключения к PostgreSQL | `postgres://user:pass@localhost:5432/db` |
| `JWT_SECRET` | Секрет для подписи JWT | `min-32-chars-secret-key-here!!!` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | `./certs` |
| `REDIS_ADDR` | Адрес Redis | `localhost:6379` |
| `ES_URL` | URL Elasticsearch | `http://localhost:9200` |
| `CLICKHOUSE_DSN` | Адрес ClickHouse | `localhost:9000` |
| `KAFKA_BROKERS` | Список брокеров Kafka | `localhost:9092` |
| `USER_SERVICE_ADDR` | Адрес user-service (gateway) | `localhost:50051` |
| `CATALOG_SERVICE_ADDR` | Адрес catalog-service (gateway) | `localhost:50052` |
| `INVENTORY_ADDR` | Адрес inventory-service (order) | `localhost:50053` |
| `PAYMENT_ADDR` | Адрес payment-service (order) | `localhost:50054` |
| `RATE_LIMIT_RPS` | RPS для rate limiter (gateway) | `10` |
| `PORT` | HTTP порт (gateway) | `8080` |
| `TRUSTED_PROXIES` | Список доверенных прокси (gateway) | `127.0.0.1,10.0.0.0/8` |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |

> **Примечание:** `LOG_LEVEL`, `LOG_FORMAT`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `METRICS_PORT` — заявлены в README сервисов, но **не реализованы** в `config.go`.

## Kubernetes

### Helm charts

Для каждого сервиса есть Helm chart в `infra/k8s/helm-charts/<service>/`.

```bash
# Установить все сервисы
for svc in api-gateway user-service catalog-service order-service inventory-service payment-service notification-service analytics-service; do
  helm upgrade --install $svc infra/k8s/helm-charts/$svc \
    --namespace marketplace \
    --create-namespace \
    --set image.tag=v0.2.0
done
```

### Структура Helm chart

```
helm-charts/<service>/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── _helpers.tpl
    ├── deployment.yaml
    ├── service.yaml
    ├── ingress.yaml
    ├── pdb.yaml          # PodDisruptionBudget
    └── secret.yaml       # Secrets (базы, JWT, TLS)
```

### HPA и PDB

- **PDB** — гарантия доступности при обновлениях (`minAvailable: 1`, создаётся при `replicaCount > 1`)
- **HPA** — упомянут в документации, но файлы `hpa.yaml` **отсутствуют** в Helm charts

### Security contexts

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
```

### Ingress

```bash
# Получить адрес
kubectl get ingress -n marketplace
```

## CI/CD

GitHub Actions выполняет:
1. **Lint** — `golangci-lint`
2. **Proto** — `buf lint`, `buf breaking`
3. **Security** — `govulncheck`
4. **Test** — unit тесты с race detector (integration тесты **не запускаются** — отсутствует `-tags=integration`)
5. **Build** — Docker images для всех сервисов
6. **Helm** — lint + template validate

Coverage gate: **60%**.

### Ручной деплой

```bash
# Сборка образа
docker build --build-arg SERVICE_NAME=api-gateway -t api-gateway:v0.2.0 -f Dockerfile .

# Пуш в registry
docker push api-gateway:v0.2.0

# Обновить Helm
helm upgrade --install api-gateway infra/k8s/helm-charts/api-gateway \
  --set image.tag=v0.2.0
```

## Мониторинг

После деплоя через Docker Compose:

| Инструмент | Доступ | Что смотреть |
|------------|--------|--------------|
| Prometheus | http://localhost:9090 | `grpc_server_handled_total` |
| Grafana | http://localhost:3000 | Dashboards |
| Jaeger | http://localhost:16686 | Distributed traces |

> **Важно:** В `infra/k8s` нет манифестов для Prometheus/Grafana/Jaeger — только в `infra/docker/docker-compose.yml`.
