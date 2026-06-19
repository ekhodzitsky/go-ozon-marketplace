use crate::circuit_breaker::CircuitBreaker;
use crate::config::Config;
use crate::error::ApiError;
use crate::proto::{
    analytics::v1 as analytics_v1, catalog::v1 as catalog_v1, inventory::v1 as inventory_v1,
    order::v1 as order_v1, payment::v1 as payment_v1, user::v1 as user_v1,
};
use std::collections::HashMap;
use std::future::Future;
use std::sync::Arc;
use std::time::Instant;
use tonic::transport::{Certificate, Channel, ClientTlsConfig, Identity as TlsIdentity};

/// Фабрика tonic-клиентов downstream-сервисов. Клиенты клонируются на каждый вызов.
/// Для каждого сервиса свой circuit breaker; вызовы идут через `call`, который пишет метрики.
#[derive(Clone)]
#[allow(dead_code)]
pub struct Clients {
    pub user: user_v1::user_service_client::UserServiceClient<Channel>,
    pub catalog: catalog_v1::catalog_service_client::CatalogServiceClient<Channel>,
    pub order: order_v1::order_service_client::OrderServiceClient<Channel>,
    pub inventory: inventory_v1::inventory_service_client::InventoryServiceClient<Channel>,
    pub payment: payment_v1::payment_service_client::PaymentServiceClient<Channel>,
    pub analytics: analytics_v1::analytics_service_client::AnalyticsServiceClient<Channel>,
    breakers: Arc<HashMap<&'static str, CircuitBreaker>>,
}

impl Clients {
    pub async fn new(cfg: &Config) -> Result<Self, ApiError> {
        let breakers: HashMap<&'static str, CircuitBreaker> = [
            ("user", CircuitBreaker::default()),
            ("catalog", CircuitBreaker::default()),
            ("order", CircuitBreaker::default()),
            ("inventory", CircuitBreaker::default()),
            ("payment", CircuitBreaker::default()),
            ("analytics", CircuitBreaker::default()),
        ]
        .into_iter()
        .collect();

        Ok(Self {
            user: user_v1::user_service_client::UserServiceClient::new(
                connect(&cfg.user_service_addr, cfg).await?,
            ),
            catalog: catalog_v1::catalog_service_client::CatalogServiceClient::new(
                connect(&cfg.catalog_service_addr, cfg).await?,
            ),
            order: order_v1::order_service_client::OrderServiceClient::new(
                connect(&cfg.order_service_addr, cfg).await?,
            ),
            inventory: inventory_v1::inventory_service_client::InventoryServiceClient::new(
                connect(&cfg.inventory_service_addr, cfg).await?,
            ),
            payment: payment_v1::payment_service_client::PaymentServiceClient::new(
                connect(&cfg.payment_service_addr, cfg).await?,
            ),
            analytics: analytics_v1::analytics_service_client::AnalyticsServiceClient::new(
                connect(&cfg.analytics_service_addr, cfg).await?,
            ),
            breakers: Arc::new(breakers),
        })
    }

    /// Обертка для gRPC-вызова: circuit breaker + метрики.
    pub async fn call<F, Fut, T>(&self, service: &'static str, fut: F) -> Result<T, ApiError>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = Result<T, tonic::Status>> + Send,
        T: Send,
    {
        let breaker = self
            .breakers
            .get(service)
            .cloned()
            .unwrap_or_else(CircuitBreaker::default);
        breaker
            .call(service, || async {
                let start = Instant::now();
                let result = fut().await;
                let status = match &result {
                    Ok(_) => "ok",
                    Err(_) => "error",
                };
                metrics::counter!(
                    "grpc_requests_total",
                    "service" => service,
                    "status" => status
                )
                .increment(1);
                metrics::histogram!(
                    "grpc_request_duration_seconds",
                    "service" => service
                )
                .record(start.elapsed().as_secs_f64());
                result.map_err(ApiError::from)
            })
            .await
    }
}

async fn connect(addr: &str, cfg: &Config) -> Result<Channel, ApiError> {
    let endpoint = tonic::transport::Endpoint::from_shared(addr.to_string())
        .map_err(|e| ApiError::Internal(e.to_string()))?;

    let endpoint = if cfg.insecure_skip_tls {
        endpoint
    } else if cfg.tls_enabled || cfg.mtls_enabled {
        let tls = build_tls_config(cfg)?;
        endpoint
            .tls_config(tls)
            .map_err(|e| ApiError::Internal(e.to_string()))?
    } else {
        endpoint
    };

    endpoint
        .connect()
        .await
        .map_err(|e| ApiError::Downstream(e.to_string()))
}

fn build_tls_config(cfg: &Config) -> Result<ClientTlsConfig, ApiError> {
    let mut tls = ClientTlsConfig::new();
    if cfg.mtls_enabled {
        let cert = std::fs::read_to_string(&cfg.cert_path)
            .map_err(|e| ApiError::Internal(format!("read cert: {e}")))?;
        let key = std::fs::read_to_string(&cfg.key_path)
            .map_err(|e| ApiError::Internal(format!("read key: {e}")))?;
        tls = tls.identity(TlsIdentity::from_pem(cert, key));
    }
    if !cfg.cert_path.is_empty() && !cfg.mtls_enabled {
        let ca = std::fs::read_to_string(&cfg.cert_path)
            .map_err(|e| ApiError::Internal(format!("read ca: {e}")))?;
        tls = tls.ca_certificate(Certificate::from_pem(ca));
    }
    Ok(tls)
}
