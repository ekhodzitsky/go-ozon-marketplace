use crate::error::ApiError;

/// Конфигурация сервиса. Все значения читаются из окружения,
/// для отсутствующих используются dev-значения, совпадающие с Go-шлюзом.
#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct Config {
    pub user_service_addr: String,
    pub catalog_service_addr: String,
    pub order_service_addr: String,
    pub inventory_service_addr: String,
    pub payment_service_addr: String,
    pub analytics_service_addr: String,
    pub http_port: u16,
    pub jwt_secret: String,
    pub cors_allowed_origins: Vec<String>,
    pub log_level: String,
}

impl Config {
    pub fn from_env() -> Result<Self, ApiError> {
        Ok(Self {
            user_service_addr: env_or("USER_SERVICE_ADDR", "localhost:50051"),
            catalog_service_addr: env_or("CATALOG_SERVICE_ADDR", "localhost:50052"),
            order_service_addr: env_or("ORDER_SERVICE_ADDR", "localhost:50055"),
            inventory_service_addr: env_or("INVENTORY_SERVICE_ADDR", "localhost:50053"),
            payment_service_addr: env_or("PAYMENT_SERVICE_ADDR", "localhost:50054"),
            analytics_service_addr: env_or("ANALYTICS_SERVICE_ADDR", "localhost:50056"),
            http_port: env_parse_or("PORT", 8080),
            jwt_secret: env_or("JWT_SECRET", "dev-secret"),
            cors_allowed_origins: parse_list(env_or("CORS_ALLOWED_ORIGINS", "")),
            log_level: env_or("RUST_LOG", "info"),
        })
    }
}

fn env_or(key: &str, default: &str) -> String {
    std::env::var(key).unwrap_or_else(|_| default.to_string())
}

fn env_parse_or<T: std::str::FromStr>(key: &str, default: T) -> T {
    std::env::var(key)
        .ok()
        .and_then(|s| s.parse().ok())
        .unwrap_or(default)
}

fn parse_list(s: String) -> Vec<String> {
    s.split(',')
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty())
        .collect()
}
