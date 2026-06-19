use crate::feature_flags::{router as flags_router, FlagStore};
use crate::metrics;
use axum::{extract::Extension, routing::get, Json, Router};
use metrics_exporter_prometheus::PrometheusHandle;
use serde_json::json;

/// Admin-роутер: health/readiness, Prometheus-метрики и управление feature flags.
pub fn router(prometheus: PrometheusHandle, flags: FlagStore) -> Router {
    Router::new()
        .route("/health", get(health))
        .route("/ready", get(ready))
        .route("/metrics", get(metrics::handler))
        .nest("/flags", flags_router(flags))
        .layer(Extension(prometheus))
}

async fn health() -> Json<serde_json::Value> {
    Json(json!({ "status": "healthy" }))
}

async fn ready() -> Json<serde_json::Value> {
    Json(json!({ "status": "ready" }))
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::Body;
    use axum::http::{Request, StatusCode};
    use metrics_exporter_prometheus::PrometheusBuilder;
    use tower::ServiceExt;

    #[tokio::test]
    async fn health_returns_ok() {
        let recorder = PrometheusBuilder::new().build_recorder();
        let app = router(recorder.handle(), FlagStore::memory());
        let response = app
            .oneshot(Request::get("/health").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(response.status(), StatusCode::OK);
    }
}
