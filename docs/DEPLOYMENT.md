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
| `HTTP_PORT` | HTTP порт (user-service) | `8080` |
| `PORT` | HTTP порт (api-gateway) | `8080` |
| `METRICS_PORT` | Порт для Prometheus метрик | `51055` |
| `POSTGRES_DSN` | Строка подключения к PostgreSQL | `postgres://user:pass@localhost:5432/db` |
| `JWT_SECRET` | Секрет для подписи JWT | `min-32-chars-secret-key-here!!!` |
| `CERT_PATH` | Путь к TLS сертификатам (опционально) | `./certs` |
| `REDIS_ADDR` | Адрес Redis | `localhost:6379` |
| `ES_URL` | URL Elasticsearch | `http://localhost:9200` |
| `CLICKHOUSE_DSN` | Адрес ClickHouse | `localhost:9000` |
| `KAFKA_BROKERS` | Список брокеров Kafka | `localhost:9092` |
| `KAFKA_TOPIC` | Топик для order-service | `order-events` |
| `KAFKA_TOPICS` | Топики через запятую (analytics, notification) | `order-events` |
| `KAFKA_CONSUMER_GROUP` | Группа потребителей Kafka | `order-service` |
| `KAFKA_DLQ_TOPIC` | Dead-letter топик (notification) | `notification-dlq` |
| `DLQ_TOPIC` | Dead-letter топик (payment) | `payment-dlq` |
| `USER_SERVICE_ADDR` | Адрес user-service (gateway) | `localhost:50051` |
| `CATALOG_SERVICE_ADDR` | Адрес catalog-service (gateway) | `localhost:50052` |
| `CATALOG_ADDR` | Адрес catalog-service (order) | `localhost:50052` |
| `INVENTORY_ADDR` | Адрес inventory-service (order/gateway) | `localhost:50053` |
| `INVENTORY_SERVICE_ADDR` | Адрес inventory-service (gateway) | `localhost:50053` |
| `PAYMENT_ADDR` | Адрес payment-service (order/gateway) | `localhost:50054` |
| `PAYMENT_SERVICE_ADDR` | Адрес payment-service (gateway) | `localhost:50054` |
| `ORDER_SERVICE_ADDR` | Адрес order-service (gateway) | `localhost:50055` |
| `ANALYTICS_SERVICE_ADDR` | Адрес analytics-service (gateway) | `localhost:50056` |
| `SMTP_HOST` | SMTP сервер (notification, опционально) | `smtp.example.com` |
| `SMTP_PORT` | SMTP порт | `587` |
| `SMTP_FROM` | Отправитель писем | `notifications@example.com` |
| `SMTP_USER` | SMTP логин | `user` |
| `SMTP_PASSWORD` | SMTP пароль | `pass` |
| `RATE_LIMIT_RPS` | RPS для rate limiter (gateway) | `10` |
| `TRUSTED_PROXIES` | Список доверенных прокси (gateway) | `127.0.0.1,10.0.0.0/8` |
| `DEFAULT_CALL_TIMEOUT` | Таймаут gRPC вызовов | `5s` |
| `DEFAULT_QUERY_TIMEOUT` | Таймаут gRPC запросов | `3s` |
| `LOG_LEVEL` | Уровень логирования | `info` |
| `LOG_FORMAT` | Формат логов | `json` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Endpoint OTLP экспортёра | `http://localhost:4318` |

## Kubernetes

Сервисы деплоятся в namespace `marketplace-staging`.

### Namespace

```bash
kubectl apply -f infra/k8s/namespace.yaml
```

### Helm charts

Для каждого сервиса есть Helm chart в `infra/k8s/helm-charts/<service>/`.

```bash
# Установить все сервисы
for svc in api-gateway user-service catalog-service order-service inventory-service payment-service notification-service analytics-service; do
  helm upgrade --install $svc infra/k8s/helm-charts/$svc \
    --namespace marketplace-staging \
    --create-namespace \
    --values infra/k8s/helm-charts/$svc/values-staging.yaml \
    --set image.tag=v0.2.0 \
    --set secrets.jwtSecret="$(openssl rand -hex 32)" \
    --set secrets.postgresDSN="postgres://user:pass@postgres:5432/db"
done
```

> **Важно:** `secrets.postgresDSN` нужен только сервисам с PostgreSQL. Для сервисов без БД переменную можно не задавать.

### Структура Helm chart

```
helm-charts/<service>/
├── Chart.yaml
├── values.yaml
├── values-staging.yaml
└── templates/
    ├── _helpers.tpl
    ├── deployment.yaml
    ├── service.yaml
    ├── ingress.yaml
    ├── hpa.yaml          # HorizontalPodAutoscaler
    ├── pdb.yaml          # PodDisruptionBudget
    └── secret.yaml       # Secrets (базы, JWT, TLS)
```

### HPA и PDB

- **PDB** — гарантия доступности при обновлениях (`minAvailable: 1`, создаётся при `replicaCount > 1`).
- **HPA** — горизонтальное авто-масштабирование по CPU/memory. Выключается по умолчанию (`autoscaling.enabled: false`).

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
kubectl get ingress -n marketplace-staging
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
  --namespace marketplace-staging \
  --values infra/k8s/helm-charts/api-gateway/values-staging.yaml \
  --set image.tag=v0.2.0
```

## Мониторинг

### Локально (Docker Compose)

| Инструмент | Доступ | Что смотреть |
|------------|--------|--------------|
| Prometheus | http://localhost:9090 | `grpc_server_handled_total` |
| Grafana | http://localhost:3000 | Dashboards |
| Jaeger | http://localhost:16686 | Distributed traces |

### В Kubernetes

Манифесты лежат в `infra/k8s/monitoring/`:

```bash
kubectl apply -f infra/k8s/monitoring/namespace.yaml
kubectl apply -f infra/k8s/monitoring/prometheus/
kubectl apply -f infra/k8s/monitoring/grafana/
kubectl apply -f infra/k8s/monitoring/jaeger/
```

Prometheus скрейпит сервисы в namespace `marketplace-staging` по аннотации `prometheus.io/scrape: "true"`. Grafana автоматически подхватывает dashboards из `infra/monitoring/grafana/dashboards/` через provisioning provider. Jaeger принимает трейсы по OTLP (порты 4317/4318).

| Инструмент | Доступ внутри кластера | Назначение |
|------------|------------------------|------------|
| Prometheus | http://prometheus.monitoring.svc.cluster.local:9090 | Метрики, алерты |
| Grafana | http://grafana.monitoring.svc.cluster.local:3000 | Дашборды |
| Jaeger | http://jaeger.monitoring.svc.cluster.local:16686 | Трейсы |
