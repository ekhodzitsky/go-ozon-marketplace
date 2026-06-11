# Развёртывание

Как запустить маркетплейс локально и в Kubernetes.

## Локальная разработка

### Через Docker Compose

```bash
# Инфраструктура + все сервисы
make up

# Проверить статус
docker compose -f infra/docker/docker-compose.yml ps

# Логи сервиса
docker compose -f infra/docker/docker-compose.yml logs -f order-service

# Остановить и удалить volumes
make down
```

### Вручную (Go)

```bash
# 1. Инфраструктура
make up

# 2. Каждый сервис в отдельном терминале
cd services/<service> && go run ./cmd/...
```

### Переменные окружения

| Переменная | Описание | Пример |
|------------|----------|--------|
| `SERVICE_NAME` | Имя сервиса | `order-service` |
| `GRPC_PORT` | Порт gRPC сервера | `50055` |
| `HTTP_PORT` | Порт HTTP сервера | `8080` |
| `POSTGRES_URL` | Строка подключения к PostgreSQL | `postgres://user:pass@localhost:5432/db` |
| `REDIS_URL` | Строка подключения к Redis | `redis://localhost:6379/0` |
| `KAFKA_BROKERS` | Список брокеров Kafka | `localhost:19092` |
| `CLICKHOUSE_URL` | Строка подключения к ClickHouse | `clickhouse://localhost:9000` |
| `ELASTICSEARCH_URL` | URL Elasticsearch | `http://localhost:9200` |
| `JWT_SECRET` | Секрет для подписи JWT | `min-32-chars-secret-key-here!!!` |
| `LOG_LEVEL` | Уровень логирования | `info` |
| `LOG_FORMAT` | Формат логов | `json` или `console` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint для трассировки | `http://localhost:4318` |
| `METRICS_PORT` | Порт для Prometheus metrics | `9090` |

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

- **HPA** — автомасштабирование по CPU/memory
- **PDB** — гарантия доступности при обновлениях (`minAvailable: 1`)

### Security contexts

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
```

### Ingress

```bash
# Получить адрес
kubectl get ingress -n marketplace

# Пример: https://marketplace.example.com/graphql
```

## CI/CD

GitHub Actions выполняет:
1. **Lint** — `golangci-lint`
2. **Proto** — `buf lint`, `buf breaking`
3. **Security** — `govulncheck`
4. **Test** — unit + integration с race detector
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

После деплоя:

| Инструмент | Доступ | Что смотреть |
|------------|--------|--------------|
| Prometheus | `kubectl port-forward svc/prometheus 9090` | `grpc_server_handled_total`, `orders_created_total` |
| Grafana | `kubectl port-forward svc/grafana 3000` | Dashboards: RED, Business metrics |
| Jaeger | `kubectl port-forward svc/jaeger 16686` | Distributed traces |
