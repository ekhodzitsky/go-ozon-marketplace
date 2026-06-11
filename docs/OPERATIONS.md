# Runbook для on-call: go-ozon-marketplace

**Версия:** 0.3.0  
**Дата обновления:** 2026-06-11  
**Аудитория:** SRE, платформенные инженеры, дежурные разработчики

---

## Быстрые ссылки

| Инструмент | URL (prod) | URL (staging) |
|------------|-----------|---------------|
| Grafana | `https://grafana.marketplace.internal` | `https://grafana-staging.marketplace.internal` |
| ArgoCD | `https://argocd.marketplace.internal` | `https://argocd-staging.marketplace.internal` |
| Prometheus | `https://prometheus.marketplace.internal` | — |
| Jaeger | `https://jaeger.marketplace.internal` | — |

---

## 1. Что делать, если order-service падает

### Симптомы
- Алерт `LowAvailability` для `order-service`
- График RED Method показывает ошибки / высокую латентность
- Клиенты не могут создать заказ

### Диагностика

```bash
# Проверить поды
kubectl get pods -n marketplace -l app.kubernetes.io/name=order-service

# Логи
kubectl logs -n marketplace -l app.kubernetes.io/name=order-service --tail=500 | jq .

# Проверить Saga state
kubectl exec -n marketplace deploy/postgres -- psql -U ozon -d marketplace -c \
  "SELECT status, COUNT(*) FROM orders WHERE updated_at > NOW() - INTERVAL '1 hour' GROUP BY status;"
```

### Saga: проверка незавершённых транзакций

```sql
-- Незавершённые саги
SELECT s.id, s.order_id, s.status, s.step, o.status as order_status
FROM sagas s
JOIN orders o ON s.order_id = o.id
WHERE s.status NOT IN ('completed', 'compensated')
  AND s.updated_at > NOW() - INTERVAL '2 hours';
```

### Ручная компенсация

Если Saga застряла и recovery worker не справляется:

```sql
-- Отменить заказ вручную
BEGIN;
UPDATE orders SET status = 'cancelled', updated_at = NOW() WHERE id = '<order_id>';
UPDATE sagas SET status = 'compensated', step = 'compensate', updated_at = NOW() WHERE order_id = '<order_id>';
COMMIT;
```

> ⚠️ Перед ручной компенсацией убедитесь, что:
> - Инвентарь освобождён: `SELECT * FROM inventory_ledger WHERE order_id = '<order_id>' AND action = 'release'`
> - Платёж возвращён: `SELECT * FROM payment_refunds WHERE order_id = '<order_id>'`

### Перезапуск

```bash
kubectl rollout restart deployment/order-service -n marketplace
```

---

## 2. Что делать, если Kafka недоступна

### Симптомы
- Алерт `KafkaConsumerLag` растёт
- Outbox relay логирует ошибки публикации
- notification-service / analytics-service не получают события

### Диагностика

```bash
# Статус брокеров
kubectl get pods -n kafka  # или redpanda

# Consumer groups
kafka-consumer-groups.sh --bootstrap-server kafka:9092 --describe --group analytics-group
kafka-consumer-groups.sh --bootstrap-server kafka:9092 --describe --group notification-group

# Outbox backlog
kubectl exec -n marketplace deploy/postgres -- psql -U ozon -d marketplace -c \
  "SELECT COUNT(*) FROM outbox WHERE processed_at IS NULL;"
```

### Проверить outbox

```sql
-- Необработанные события
SELECT event_type, COUNT(*), MIN(created_at), MAX(retry_count)
FROM outbox
WHERE processed_at IS NULL
GROUP BY event_type;

-- DLQ
SELECT event_type, error, COUNT(*) FROM outbox_dlq GROUP BY event_type, error;
```

### Ручной relay

Если outbox relay не работает, можно запустить ручную доставку:

```bash
# Подключиться к поду order-service
kubectl exec -n marketplace deploy/order-service -it -- sh

# Проверить метрики relay
wget -qO- localhost:9100/metrics | grep outbox
```

При необходимости — массовая пометка событий для повторной обработки:

```sql
UPDATE outbox SET processed_at = NULL, retry_count = 0
WHERE processed_at IS NULL AND retry_count >= 5;
```

---

## 3. Что делать, если Redis падает

### Симптомы
- Алерт `RedisMemoryHigh`
- Rate limiter перестаёт ограничивать (`fail open`)
- inventory-service кэш-промахи растут

### Диагностика

```bash
kubectl get pods -n marketplace -l app.kubernetes.io/name=redis
kubectl logs -n marketplace -l app.kubernetes.io/name=redis --tail=100
```

### Rate limiter fallback

Rate limiter спроектирован с `fail open`: если Redis недоступен, запросы пропускаются.

```go
// pkg/middleware/ratelimit.go
if err != nil {
    return true // fail open
}
```

**Действия:**
1. Проверить коннект из api-gateway: `redis-cli -h redis ping`
2. Если Redis упал — восстановить под / StatefulSet
3. Мониторить задолженность: запросы не дропаются, но лимитов нет — включить emergency rate limiting на ingress-nginx при необходимости

### Inventory cache fallback

inventory-service при промахе идёт в PostgreSQL (`cache-aside`).

```bash
# Проверить hit ratio в Grafana dashboard "Infrastructure"
# Если ratio < 50% — Redis не работает или ключи протухли
```

---

## 4. Что делать, если circuit breaker открылся

### Симптомы
- Алерт `HighErrorRate` в downstream-сервисе
- `api-gateway` возвращает ошибки на всех запросах к одному сервису
- Метрика `circuitbreaker_state{state="open"}` = 1

### Диагностика

```bash
# Проверить состояние CB
kubectl logs -n marketplace -l app.kubernetes.io/name=api-gateway --tail=200 | grep -i "circuit"

# Проверить целевой сервис
kubectl get pods -n marketplace -l app.kubernetes.io/name=<downstream-service>
kubectl logs -n marketplace -l app.kubernetes.io/name=<downstream-service> --tail=500
```

### Проверить downstream

1. **gRPC health check:**
```bash
grpcurl -plaintext <downstream-service>:50051 grpc.health.v1.Health/Check
```

2. **Метрики latency / errors:**
```promql
histogram_quantile(0.99, rate(grpc_server_handling_seconds_bucket{service="<downstream>"}[5m]))
rate(grpc_server_handled_total{service="<downstream>",status!="OK"}[5m])
```

### Ручной reset

Circuit breaker автоматически переходит в `HalfOpen` через 30 секунд. Для ручного сброса:

```bash
# Перезапуск api-gateway сбрасывает состояние CB (in-memory)
kubectl rollout restart deployment/api-gateway -n marketplace
```

> ⚠️ Не сбрасывайте CB, если downstream всё ещё нездоров — это вызвет каскадный отказ.

---

## 5. Как откатить релиз

### Способ A: ArgoCD Rollback

1. Открыть `https://argocd.marketplace.internal`
2. Application `marketplace` → `History and Rollback`
3. Выбрать предыдущую ревизию → `Rollback`
4. Подтвердить sync

### Способ B: Helm Rollback

```bash
# Список релизов
helm list -n marketplace

# Откат на предыдущую ревизию
helm rollback <release-name> 0 -n marketplace

# Или на конкретную ревизию
helm rollback api-gateway 42 -n marketplace
```

### Способ C: kubectl rollout undo

```bash
kubectl rollout undo deployment/<service> -n marketplace
# Или на конкретную ревизию
kubectl rollout undo deployment/<service> -n marketplace --to-revision=3
```

### Проверка после отката

```bash
kubectl get pods -n marketplace
kubectl rollout status deployment/<service> -n marketplace
```

---

## 6. Как проверить SLO

### Дашборды

| SLO | Grafana Dashboard | Panel |
|-----|------------------|-------|
| Доступность 99.9% | RED Method | Uptime |
| Латентность p99 < 500ms | RED Method | Duration p99 |
| Error Rate < 1% | RED Method | Error Rate (%) |
| Бизнес: заказы | Business Metrics | Orders Created/Confirmed |
| Инфраструктура | Infrastructure | CPU, Memory, Connections |

### PromQL для ручной проверки

```promql
# Доступность за последний час
avg_over_time(up{namespace="marketplace"}[1h])

# Error rate by service
sum(rate(grpc_server_handled_total{status!="OK"}[5m])) by (service)
  /
sum(rate(grpc_server_handled_total[5m])) by (service)

# P99 latency by service
histogram_quantile(0.99,
  sum(rate(grpc_server_handling_seconds_bucket[5m])) by (le, service)
)

# Kafka consumer lag
kafka_consumer_group_lag{namespace="marketplace"}

# Redis hit ratio
rate(redis_keyspace_hits_total[1m])
  /
(rate(redis_keyspace_hits_total[1m]) + rate(redis_keyspace_misses_total[1m]))
```

---

## 7. Как масштабировать

### Автоматическое масштабирование (HPA)

Включено для всех сервисов. Параметры по умолчанию:

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
  targetMemoryUtilizationPercentage: 80
```

Проверить статус:

```bash
kubectl get hpa -n marketplace
```

### Ручное масштабирование

```bash
# Увеличить реплики
kubectl scale deployment/api-gateway --replicas=5 -n marketplace

# Или через Helm
helm upgrade api-gateway ./infra/k8s/helm-charts/api-gateway \
  -n marketplace \
  --set replicaCount=5
```

### Вертикальное масштабирование

```bash
# Patch resources
kubectl patch deployment api-gateway -n marketplace -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"api-gateway","resources":{"limits":{"cpu":"2000m","memory":"2Gi"}}}]}}}}'
```

### Масштабирование БД

- **PostgreSQL**: Connection pool `MaxConns=20`, `MinConns=5`. При росте нагрузки масштабировать read replicas.
- **Redis**: Cluster mode или Sentinel для HA.
- **Kafka**: Увеличить partitions для топика `orders`.
- **Elasticsearch**: Добавить data nodes.
- **ClickHouse**: Horizontal scaling через distributed tables.

---

## 8. Emergency contacts & escalation matrix

### Severity Levels

| Уровень | Описание | Время реакции | Действие |
|---------|----------|---------------|----------|
| **SEV-1** | Полный downtime маркетплейса | 15 мин | Страница on-call, war room |
| **SEV-2** | Деградация core flow (заказы, оплата) | 30 мин | Страница on-call |
| **SEV-3** | Деградация non-core (аналитика, email) | 2 часа | Тикет, след. рабочий день OK |
| **SEV-4** | Предупреждение (capacity, лаги) | 24 часа | Плановая задача |

### Escalation

```
1. On-call SRE (PagerDuty / OpsGenie)
   └─ Нет ответа за 15 мин → Tech Lead команды
      └─ Нет ответа за 30 мин → Engineering Manager
         └─ Нет ответа за 1 час → CTO
```

### Контакты

| Роль | Контакт | Метод |
|------|---------|-------|
| On-call SRE | `#alerts-marketplace` (Slack) / PagerDuty | Страница |
| Backend Platform | `@platform-backend` | Slack |
| Data Platform | `@platform-data` | Slack |
| Security | `@security-team` | Slack / Email: security@company.internal |

### War room

- **Slack:** `#incident-marketplace`
- **Meet:** `https://meet.company.internal/incident-marketplace`
- **Шаблон post-mortem:** `docs/postmortem-TEMPLATE.md` (создать при первом инциденте)

---

## Чек-лист для смены

- [ ] Grafana dashboards без аномалий
- [ ] Нет активных алертов severity ≥ warning
- [ ] Kafka consumer lag < 1000
- [ ] Redis hit ratio > 80%
- [ ] Все поды Running
- [ ] ArgoCD sync в Healthy
- [ ] Последний deploy стабилен > 1 час
