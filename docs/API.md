# API

Как взаимодействовать с маркетплейсом: GraphQL для клиентов, gRPC между сервисами.

## GraphQL Gateway

Единая точка входа — `api-gateway` на порту `8080`.

### Мутации

| Мутация | Описание | Авторизация |
|---------|----------|-------------|
| `register` | Регистрация нового пользователя | Нет |
| `login` | Вход, получение JWT | Нет |
| `createProduct` | Создать товар | `admin` |
| `updateProduct` | Обновить товар | `admin` |
| `deleteProduct` | Удалить товар | `admin` |
| `createOrder` | Оформить заказ | `user` |
| `cancelOrder` | Отменить заказ | `user` (свой) / `admin` |

### Запросы

| Запрос | Описание | Авторизация |
|--------|----------|-------------|
| `me` | Текущий пользователь | `user` |
| `user(id)` | Пользователь по ID | `admin` |
| `products(filter, pagination)` | Список товаров | Нет |
| `product(id)` | Товар по ID | Нет |
| `searchProducts(query)` | Поиск в Elasticsearch | Нет |
| `order(id)` | Заказ по ID | `user` (свой) / `admin` |
| `orders(userId, status)` | Список заказов | `user` (свои) / `admin` |
| `inventory(productId)` | Остатки товара | Нет |

### Пример запроса

```graphql
query {
  products(filter: { categoryId: "1" }, pagination: { limit: 10, offset: 0 }) {
    items {
      id
      name
      price
      stock
    }
    total
  }
}
```

### Авторизация

Передайте JWT в заголовке:

```
Authorization: Bearer <token>
```

Токен выдаётся при `register` или `login`. Содержит:
- `sub` — user ID
- `role` — `user`, `admin` или `service`
- `iss`, `aud`, `jti`, `nbf`, `exp`

## gRPC контракты

Все сервисы общаются через gRPC. Proto-файлы лежат в `api/proto/`.

| Сервис | Proto файл | Ключевые методы |
|--------|-----------|----------------|
| **user-service** | `user/v1/user.proto` | `Register`, `Login`, `GetUser`, `ValidateToken` |
| **catalog-service** | `catalog/v1/catalog.proto` | `CreateProduct`, `GetProduct`, `UpdateProduct`, `DeleteProduct`, `SearchProducts`, `ListCategories` |
| **order-service** | `order/v1/order.proto` | `CreateOrder`, `GetOrder`, `CancelOrder`, `ListOrders`, `UpdateOrderStatus` |
| **inventory-service** | `inventory/v1/inventory.proto` | `GetStock`, `ReserveStock`, `ReleaseStock`, `UpdateStock` |
| **payment-service** | `payment/v1/payment.proto` | `ProcessPayment`, `RefundPayment`, `GetPaymentStatus` |
| **notification-service** | `notification/v1/notification.proto` | `SendNotification` (service-only) |
| **analytics-service** | `analytics/v1/analytics.proto` | `RecordEvent`, `GetReport` (service-only) |

### Генерация кода из proto

```bash
make proto
```

Генерирует Go-код в `api/gen/go/`.

### Требования к gRPC вызовам

- Все вызовы между сервисами — с mTLS
- `traceparent` передаётся через gRPC metadata для трассировки
- Deadline: 5 секунд для write, 3 секунды для read
