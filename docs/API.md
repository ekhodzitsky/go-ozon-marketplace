# API

Как взаимодействовать с маркетплейсом: GraphQL для клиентов, gRPC между сервисами.

## GraphQL Gateway

Единая точка входа — `api-gateway` на порту `8080`.

> **Важно:** Gateway **не валидирует** JWT самостоятельно — он только прокидывает заголовок `Authorization` в gRPC metadata downstream сервисам. Валидация происходит в gRPC interceptors каждого сервиса.

### Мутации

| Мутация | Сигнатура | Описание |
|---------|-----------|----------|
| `register` | `register(email: String!, password: String!, name: String!): ID!` | Регистрация нового пользователя |
| `login` | `login(email: String!, password: String!): String!` | Вход, получение JWT |
| `createProduct` | `createProduct(name: String!, description: String!, price: Float!, categories: [String!]!): ID!` | Создать товар |

### Запросы

| Запрос | Сигнатура | Описание |
|--------|-----------|----------|
| `user` | `user(id: ID!): User` | Пользователь по ID |
| `product` | `product(id: ID!): Product` | Товар по ID |
| `searchProducts` | `searchProducts(query: String!, page: Int, pageSize: Int): ProductConnection` | Поиск в Elasticsearch |

### Типы

```graphql
type User {
  id: ID!
  email: String!
  name: String!
  createdAt: String!
}

type Product {
  id: ID!
  name: String!
  description: String!
  price: Float!
  categories: [String!]!
  createdAt: String!
}

type ProductConnection {
  products: [Product!]!
  total: Int!
}
```

### Авторизация

Передайте JWT в заголовке:

```
Authorization: Bearer <token>
```

> В GraphQL gateway **нет проверки ролей** — они проверяются на уровне gRPC сервисов (например, `inventory-service` и `notification-service` требуют `service` роль).

## gRPC контракты

Все сервисы общаются через gRPC. Proto-файлы лежат в `api/proto/`.

| Сервис | Proto файл | Методы |
|--------|-----------|--------|
| **user-service** | `user/v1/user.proto` | `Register`, `Login`, `GetUser` |
| **catalog-service** | `catalog/v1/catalog.proto` | `CreateProduct`, `GetProduct`, `ListProducts`, `SearchProducts` |
| **order-service** | `order/v1/order.proto` | `CreateOrder`, `GetOrder`, `ListOrders` |
| **inventory-service** | `inventory/v1/inventory.proto` | `Reserve`, `Release`, `GetStock` |
| **payment-service** | `payment/v1/payment.proto` | `ProcessPayment`, `Refund` |
| **notification-service** | `notification/v1/notification.proto` | `SendEmail` |
| **analytics-service** | `analytics/v1/analytics.proto` | `TrackEvent`, `GetDailyRevenue` |

### Генерация кода из proto

```bash
make proto
```

Генерирует Go-код в `api/gen/go/`.

### Требования к gRPC вызовам

- Вызовы между сервисами могут использовать **mTLS** при наличии `CERT_PATH` (иначе insecure)
- Deadline: 5 секунд для write, 3 секунды для read (application-level timeouts)

## JWT

Токен выдаётся при `login` (не при `register`).

**Реальные поля в токене:**
- `user_id` — ID пользователя
- `exp` — время истечения (24 часа)
- `role` — записывается если передан (по умолчанию `user`)

**Не реализовано:** `iss`, `aud`, `jti`, `nbf`, `sub`.
