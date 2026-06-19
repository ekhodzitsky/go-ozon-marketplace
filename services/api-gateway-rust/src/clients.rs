use crate::config::Config;
use crate::error::ApiError;
use crate::proto::{
    analytics::v1 as analytics_v1,
    catalog::v1 as catalog_v1,
    inventory::v1 as inventory_v1,
    order::v1 as order_v1,
    payment::v1 as payment_v1,
    user::v1 as user_v1,
};
use tonic::transport::Channel;

/// Фабрика tonic-клиентов downstream-сервисов. Клиенты клонируются на каждый вызов.
#[derive(Clone)]
#[allow(dead_code)]
pub struct Clients {
    pub user: user_v1::user_service_client::UserServiceClient<Channel>,
    pub catalog: catalog_v1::catalog_service_client::CatalogServiceClient<Channel>,
    pub order: order_v1::order_service_client::OrderServiceClient<Channel>,
    pub inventory: inventory_v1::inventory_service_client::InventoryServiceClient<Channel>,
    pub payment: payment_v1::payment_service_client::PaymentServiceClient<Channel>,
    pub analytics: analytics_v1::analytics_service_client::AnalyticsServiceClient<Channel>,
}

impl Clients {
    pub async fn new(cfg: &Config) -> Result<Self, ApiError> {
        Ok(Self {
            user: user_v1::user_service_client::UserServiceClient::new(
                connect(&cfg.user_service_addr).await?,
            ),
            catalog: catalog_v1::catalog_service_client::CatalogServiceClient::new(
                connect(&cfg.catalog_service_addr).await?,
            ),
            order: order_v1::order_service_client::OrderServiceClient::new(
                connect(&cfg.order_service_addr).await?,
            ),
            inventory: inventory_v1::inventory_service_client::InventoryServiceClient::new(
                connect(&cfg.inventory_service_addr).await?,
            ),
            payment: payment_v1::payment_service_client::PaymentServiceClient::new(
                connect(&cfg.payment_service_addr).await?,
            ),
            analytics: analytics_v1::analytics_service_client::AnalyticsServiceClient::new(
                connect(&cfg.analytics_service_addr).await?,
            ),
        })
    }
}

async fn connect(addr: &str) -> Result<Channel, ApiError> {
    let endpoint = tonic::transport::Endpoint::from_shared(addr.to_string())
        .map_err(|e| ApiError::Internal(e.to_string()))?;
    endpoint
        .connect()
        .await
        .map_err(|e| ApiError::Downstream(e.to_string()))
}
