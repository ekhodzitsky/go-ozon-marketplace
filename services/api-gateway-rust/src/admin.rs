use axum::{routing::get, Json, Router};
use serde_json::json;

/// Простейший admin-роутер. Сейчас содержит только health/readiness.
pub fn router() -> Router {
    Router::new()
        .route("/health", get(health))
        .route("/ready", get(ready))
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
    use tower::ServiceExt;

    #[tokio::test]
    async fn health_returns_ok() {
        let app = router();
        let response = app
            .oneshot(Request::get("/health").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(response.status(), StatusCode::OK);
    }
}
