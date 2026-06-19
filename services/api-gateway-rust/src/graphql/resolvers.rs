use crate::auth::{require_identity, Identity};
use crate::clients::Clients;
use crate::error::ApiError;
use crate::proto::{
    catalog::v1 as catalog_v1, order::v1 as order_v1, user::v1 as user_v1,
};
use async_graphql::{Context, InputObject, Object, Result, SimpleObject, ID};
use tonic::metadata::MetadataValue;
use tonic::Request;

#[derive(SimpleObject, Clone, Debug)]
#[graphql(rename_fields = "camelCase")]
pub struct User {
    pub id: ID,
    pub email: String,
    pub name: String,
    pub created_at: String,
}

#[derive(SimpleObject, Clone, Debug)]
#[graphql(rename_fields = "camelCase")]
pub struct Product {
    pub id: ID,
    pub name: String,
    pub description: String,
    pub price: f64,
    pub categories: Vec<String>,
    pub created_at: String,
}

#[derive(SimpleObject, Clone, Debug)]
#[graphql(rename_fields = "camelCase")]
pub struct ProductConnection {
    pub products: Vec<Product>,
    pub total: i32,
}

#[derive(SimpleObject, Clone, Debug)]
#[graphql(rename_fields = "camelCase")]
pub struct OrderItem {
    pub product_id: ID,
    pub quantity: i32,
    pub price: f64,
}

#[derive(InputObject, Clone, Debug)]
#[graphql(rename_fields = "camelCase")]
pub struct OrderItemInput {
    pub product_id: ID,
    pub quantity: i32,
    pub price: f64,
}

#[derive(SimpleObject, Clone, Debug)]
#[graphql(rename_fields = "camelCase")]
pub struct Order {
    pub id: ID,
    pub user_id: ID,
    pub items: Vec<OrderItem>,
    pub total_amount: f64,
    pub status: String,
    pub created_at: String,
    pub updated_at: String,
}

#[derive(SimpleObject, Clone, Debug)]
#[graphql(rename_fields = "camelCase")]
pub struct OrderConnection {
    pub orders: Vec<Order>,
    pub total: i32,
}

#[derive(SimpleObject, Clone, Debug)]
#[graphql(rename_fields = "camelCase")]
#[allow(dead_code)]
pub struct Inventory {
    pub product_id: ID,
    pub available: i32,
    pub reserved: i32,
}

pub struct Query;
pub struct Mutation;

fn ctx_clients<'a>(ctx: &'a Context<'a>) -> Result<&'a Clients, ApiError> {
    ctx.data::<Clients>()
        .map_err(|_| ApiError::Internal("clients not found".into()))
}

fn with_auth<T>(req: Request<T>, identity: Option<&Identity>) -> Request<T> {
    let mut req = req;
    if let Some(id) = identity {
        let value = format!("Bearer {}", id.token);
        if let Ok(v) = MetadataValue::try_from(value) {
            req.metadata_mut().insert("authorization", v);
        }
    }
    req
}

#[Object]
impl Query {
    /// Профиль текущего авторизованного пользователя.
    async fn me(&self, ctx: &Context<'_>) -> Result<Option<User>> {
        let identity = require_identity(ctx)?;
        let clients = ctx_clients(ctx)?;

        let mut client = clients.user.clone();
        let req = with_auth(Request::new(user_v1::GetUserRequest::default()), Some(identity));
        let resp = client.get_user(req).await?;
        let u = resp.into_inner();

        Ok(Some(User {
            id: u.user_id.into(),
            email: u.email,
            name: u.name,
            created_at: u.created_at,
        }))
    }

    /// Профиль пользователя по id.
    async fn user(&self, ctx: &Context<'_>, id: ID) -> Result<Option<User>> {
        let identity = ctx.data::<Identity>().ok();
        let clients = ctx_clients(ctx)?;

        let mut client = clients.user.clone();
        let req = with_auth(
            Request::new(user_v1::GetUserRequest {
                user_id: id.to_string(),
            }),
            identity,
        );
        let resp = client.get_user(req).await?;
        let u = resp.into_inner();

        Ok(Some(User {
            id: u.user_id.into(),
            email: u.email,
            name: u.name,
            created_at: u.created_at,
        }))
    }

    /// Товар по id.
    async fn product(&self, ctx: &Context<'_>, id: ID) -> Result<Option<Product>> {
        let identity = ctx.data::<Identity>().ok();
        let clients = ctx_clients(ctx)?;

        let mut client = clients.catalog.clone();
        let req = with_auth(
            Request::new(catalog_v1::GetProductRequest {
                product_id: id.to_string(),
            }),
            identity,
        );
        let resp = client.get_product(req).await?;
        Ok(resp.into_inner().product.map(proto_product_to_model))
    }

    /// Список товаров с постраничкой. Если передан query — выполняется поиск.
    async fn products(
        &self,
        ctx: &Context<'_>,
        query: Option<String>,
        page: Option<i32>,
        page_size: Option<i32>,
    ) -> Result<ProductConnection> {
        let identity = ctx.data::<Identity>().ok();
        let clients = ctx_clients(ctx)?;

        let page = page.filter(|&p| p > 0).unwrap_or(1);
        let page_size = page_size.filter(|&s| s > 0 && s <= 100).unwrap_or(10);

        let (products, total) = if let Some(q) = query.filter(|s| !s.is_empty()) {
            let mut client = clients.catalog.clone();
            let req = with_auth(
                Request::new(catalog_v1::SearchProductsRequest {
                    query: q,
                    page,
                    page_size,
                }),
                identity,
            );
            let resp = client.search_products(req).await?.into_inner();
            (resp.products, resp.total)
        } else {
            let mut client = clients.catalog.clone();
            let req = with_auth(
                Request::new(catalog_v1::ListProductsRequest { page, page_size }),
                identity,
            );
            let resp = client.list_products(req).await?.into_inner();
            (resp.products, resp.total)
        };

        Ok(ProductConnection {
            products: products.into_iter().map(proto_product_to_model).collect(),
            total,
        })
    }

    /// Заказ по id.
    async fn order(&self, ctx: &Context<'_>, id: ID) -> Result<Option<Order>> {
        let identity = require_identity(ctx)?;
        let clients = ctx_clients(ctx)?;

        let mut client = clients.order.clone();
        let req = with_auth(
            Request::new(order_v1::GetOrderRequest {
                order_id: id.to_string(),
            }),
            Some(identity),
        );
        let resp = client.get_order(req).await?;
        Ok(resp.into_inner().order.map(proto_order_to_model))
    }

    /// Список заказов пользователя.
    async fn orders(
        &self,
        ctx: &Context<'_>,
        user_id: ID,
        page: Option<i32>,
        page_size: Option<i32>,
    ) -> Result<OrderConnection> {
        let identity = require_identity(ctx)?;
        let clients = ctx_clients(ctx)?;

        let _ = user_id; // в Go-реализации авторизация owner/admin, здесь пока проксируем запрос
        let page = page.filter(|&p| p > 0).unwrap_or(1);
        let page_size = page_size.filter(|&s| s > 0 && s <= 100).unwrap_or(10);

        let mut client = clients.order.clone();
        let req = with_auth(
            Request::new(order_v1::ListOrdersRequest { page, page_size }),
            Some(identity),
        );
        let resp = client.list_orders(req).await?.into_inner();

        Ok(OrderConnection {
            orders: resp.orders.into_iter().map(proto_order_to_model).collect(),
            total: resp.total,
        })
    }
}

#[Object]
impl Mutation {
    /// Регистрация нового пользователя.
    async fn register(
        &self,
        ctx: &Context<'_>,
        email: String,
        password: String,
        name: String,
    ) -> Result<ID> {
        let clients = ctx_clients(ctx)?;
        let mut client = clients.user.clone();
        let req = Request::new(user_v1::RegisterRequest {
            email,
            password,
            name,
        });
        let resp = client.register(req).await?;
        Ok(resp.into_inner().user_id.into())
    }

    /// Вход по email и паролю. Возвращает JWT-токен.
    async fn login(&self, ctx: &Context<'_>, email: String, password: String) -> Result<String> {
        let clients = ctx_clients(ctx)?;
        let mut client = clients.user.clone();
        let req = Request::new(user_v1::LoginRequest { email, password });
        let resp = client.login(req).await?;
        Ok(resp.into_inner().token)
    }

    /// Создание товара. Требует admin-роль.
    async fn create_product(
        &self,
        ctx: &Context<'_>,
        name: String,
        description: String,
        price: f64,
        categories: Vec<String>,
    ) -> Result<ID> {
        let identity = require_identity(ctx)?;
        if !identity.is_admin() {
            return Err(ApiError::Forbidden.into());
        }
        let clients = ctx_clients(ctx)?;

        let mut client = clients.catalog.clone();
        let req = with_auth(
            Request::new(catalog_v1::CreateProductRequest {
                name,
                description,
                price_cents: dollars_to_cents(price),
                categories,
                idempotency_key: uuid::Uuid::new_v4().to_string(),
            }),
            Some(identity),
        );
        let resp = client.create_product(req).await?;
        Ok(resp.into_inner().product_id.into())
    }

    /// Создание заказа.
    async fn create_order(&self, ctx: &Context<'_>, items: Vec<OrderItemInput>) -> Result<ID> {
        let identity = require_identity(ctx)?;
        if items.is_empty() {
            return Err(ApiError::InvalidArgument("empty order".into()).into());
        }
        let clients = ctx_clients(ctx)?;

        let proto_items: Vec<order_v1::OrderItem> = items
            .into_iter()
            .map(|i| order_v1::OrderItem {
                product_id: i.product_id.to_string(),
                quantity: i.quantity,
                price_cents: dollars_to_cents(i.price),
            })
            .collect();

        let mut client = clients.order.clone();
        let req = with_auth(
            Request::new(order_v1::CreateOrderRequest {
                items: proto_items,
                idempotency_key: uuid::Uuid::new_v4().to_string(),
            }),
            Some(identity),
        );
        let resp = client.create_order(req).await?;
        Ok(resp.into_inner().order_id.into())
    }

    /// Отмена заказа.
    async fn cancel_order(&self, ctx: &Context<'_>, order_id: ID) -> Result<bool> {
        let identity = require_identity(ctx)?;
        let clients = ctx_clients(ctx)?;

        let mut client = clients.order.clone();
        let req = with_auth(
            Request::new(order_v1::CancelOrderRequest {
                order_id: order_id.to_string(),
            }),
            Some(identity),
        );
        client.cancel_order(req).await?;
        Ok(true)
    }
}

fn proto_product_to_model(p: catalog_v1::Product) -> Product {
    Product {
        id: p.product_id.into(),
        name: p.name,
        description: p.description,
        price: cents_to_dollars(p.price_cents),
        categories: p.categories,
        created_at: p.created_at,
    }
}

fn proto_order_to_model(o: order_v1::Order) -> Order {
    Order {
        id: o.order_id.into(),
        user_id: o.user_id.into(),
        items: o
            .items
            .into_iter()
            .map(|i| OrderItem {
                product_id: i.product_id.into(),
                quantity: i.quantity,
                price: cents_to_dollars(i.price_cents),
            })
            .collect(),
        total_amount: cents_to_dollars(o.total_amount_cents),
        status: order_status_to_string(o.status),
        created_at: o.created_at,
        updated_at: o.updated_at,
    }
}

fn order_status_to_string(s: i32) -> String {
    match s {
        1 => "pending",
        2 => "awaiting_payment",
        3 => "paid",
        4 => "processing",
        5 => "shipped",
        6 => "delivered",
        7 => "cancelled",
        8 => "refunded",
        _ => "",
    }
    .into()
}

fn dollars_to_cents(d: f64) -> i64 {
    (d * 100.0).round() as i64
}

fn cents_to_dollars(c: i64) -> f64 {
    c as f64 / 100.0
}
