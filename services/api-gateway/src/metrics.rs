use axum::{
    body::Body,
    extract::{Extension, Request},
    middleware::Next,
    response::{IntoResponse, Response},
};
use metrics_exporter_prometheus::{PrometheusBuilder, PrometheusHandle};
use std::time::Instant;

use crate::error::ApiError;

/// Устанавливает глобальный Prometheus recorder и возвращает handle для рендера.
pub fn setup() -> Result<PrometheusHandle, ApiError> {
    PrometheusBuilder::new()
        .install_recorder()
        .map_err(|e| ApiError::Internal(e.to_string()))
}

/// Middleware: считает http_requests_total и http_request_duration_seconds.
pub async fn track_metrics(request: Request<Body>, next: Next) -> Response {
    let start = Instant::now();
    let method = request.method().as_str().to_string();
    let path = request.uri().path().to_string();

    let response = next.run(request).await;

    let status = response.status().as_u16().to_string();
    let duration = start.elapsed().as_secs_f64();

    metrics::counter!(
        "http_requests_total",
        "method" => method.clone(),
        "status" => status.clone(),
        "path" => path.clone()
    )
    .increment(1);
    metrics::histogram!(
        "http_request_duration_seconds",
        "method" => method,
        "path" => path
    )
    .record(duration);

    response
}

/// Handler для `/admin/metrics`.
pub async fn handler(Extension(handle): Extension<PrometheusHandle>) -> impl IntoResponse {
    let body = handle.render();
    ([("content-type", "text/plain; charset=utf-8")], body)
}
