use crate::auth::require_identity;
use crate::error::ApiError;
use crate::graphql::resolvers::{Inventory, Order};
use async_graphql::{Context, Result, Subscription, ID};
use futures::stream::{Stream, StreamExt};
use serde::Deserialize;
use tokio::sync::mpsc;
use tokio_stream::wrappers::ReceiverStream;

pub struct SubscriptionRoot;

#[Subscription]
impl SubscriptionRoot {
    /// Подписка на изменения статуса заказа. Слушает Redis-канал `order-events:{orderId}`.
    async fn order_status<'a>(
        &self,
        ctx: &'a Context<'a>,
        order_id: ID,
    ) -> Result<impl Stream<Item = Result<Order>> + 'a> {
        let _identity = require_identity(ctx)?;
        let client = ctx
            .data::<redis::Client>()
            .map_err(|_| ApiError::Internal("redis client not found".into()))?;
        let order_id = order_id.to_string();
        Ok(order_status_stream(client.clone(), order_id).await?)
    }

    /// Подписка на изменения остатков товара. Слушает Redis-канал `inventory-events:{productId}`.
    async fn inventory_changed<'a>(
        &self,
        ctx: &'a Context<'a>,
        product_id: ID,
    ) -> Result<impl Stream<Item = Result<Inventory>> + 'a> {
        let _identity = require_identity(ctx)?;
        let client = ctx
            .data::<redis::Client>()
            .map_err(|_| ApiError::Internal("redis client not found".into()))?;
        let product_id = product_id.to_string();
        Ok(inventory_changed_stream(client.clone(), product_id).await?)
    }
}

#[derive(Deserialize, Debug)]
struct OrderEvent {
    #[allow(dead_code)]
    topic: String,
    #[allow(dead_code)]
    user_id: Option<String>,
    payload: OrderPayload,
}

#[derive(Deserialize, Debug)]
struct OrderPayload {
    order_id: String,
    status: String,
    #[allow(dead_code)]
    user_id: Option<String>,
}

impl From<OrderPayload> for Order {
    fn from(p: OrderPayload) -> Self {
        Order {
            id: p.order_id.into(),
            user_id: p.user_id.unwrap_or_default().into(),
            items: vec![],
            total_amount: 0.0,
            status: p.status,
            created_at: String::new(),
            updated_at: String::new(),
        }
    }
}

#[derive(Deserialize, Debug)]
struct InventoryEvent {
    #[allow(dead_code)]
    topic: String,
    payload: InventoryPayload,
}

#[derive(Deserialize, Debug)]
struct InventoryPayload {
    product_id: String,
    available: i32,
    reserved: i32,
}

impl From<InventoryPayload> for Inventory {
    fn from(p: InventoryPayload) -> Self {
        Inventory {
            product_id: p.product_id.into(),
            available: p.available,
            reserved: p.reserved,
        }
    }
}

async fn order_status_stream(
    client: redis::Client,
    order_id: String,
) -> Result<impl Stream<Item = Result<Order>>> {
    let channel = format!("order-events:{order_id}");
    let (tx, rx) = mpsc::channel::<Result<Order>>(16);

    tokio::spawn(async move {
        let mut pubsub = match pubsub_from_client(&client, &channel).await {
            Ok(p) => p,
            Err(e) => {
                let _ = tx.send(Err(e.into())).await;
                return;
            }
        };

        let mut stream = pubsub.on_message();
        while let Some(msg) = stream.next().await {
            let payload: String = msg.get_payload().unwrap_or_default();
            if let Ok(event) = serde_json::from_str::<OrderEvent>(&payload) {
                if event.payload.order_id == order_id {
                    if tx.send(Ok(event.payload.into())).await.is_err() {
                        break;
                    }
                }
            }
        }
    });

    Ok(ReceiverStream::new(rx))
}

async fn inventory_changed_stream(
    client: redis::Client,
    product_id: String,
) -> Result<impl Stream<Item = Result<Inventory>>> {
    let channel = format!("inventory-events:{product_id}");
    let (tx, rx) = mpsc::channel::<Result<Inventory>>(16);

    tokio::spawn(async move {
        let mut pubsub = match pubsub_from_client(&client, &channel).await {
            Ok(p) => p,
            Err(e) => {
                let _ = tx.send(Err(e.into())).await;
                return;
            }
        };

        let mut stream = pubsub.on_message();
        while let Some(msg) = stream.next().await {
            let payload: String = msg.get_payload().unwrap_or_default();
            if let Ok(event) = serde_json::from_str::<InventoryEvent>(&payload) {
                if event.payload.product_id == product_id {
                    if tx.send(Ok(event.payload.into())).await.is_err() {
                        break;
                    }
                }
            }
        }
    });

    Ok(ReceiverStream::new(rx))
}

async fn pubsub_from_client(
    client: &redis::Client,
    channel: &str,
) -> Result<redis::aio::PubSub, ApiError> {
    let mut pubsub = client.get_async_pubsub().await.map_err(ApiError::from)?;
    pubsub.subscribe(channel).await.map_err(ApiError::from)?;
    Ok(pubsub)
}
