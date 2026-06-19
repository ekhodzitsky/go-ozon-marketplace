mod admin;
mod auth;
mod circuit_breaker;
mod clients;
mod config;
mod error;
mod feature_flags;
mod graphql;
mod metrics;
mod proto;
mod ratelimit;
mod validation;
mod ws;

use crate::error::ApiError;
use async_graphql::http::GraphiQLSource;
use axum::{
    extract::Extension,
    middleware::{from_fn, from_fn_with_state},
    response::IntoResponse,
    routing::get,
    Router,
};
use std::net::SocketAddr;
use tower_http::cors::{AllowOrigin, Any, CorsLayer};

fn build_cors_layer(cfg: &config::Config) -> Result<CorsLayer, ApiError> {
    if cfg.cors_allowed_origins.is_empty() || cfg.cors_allowed_origins.iter().any(|o| o == "*") {
        Ok(CorsLayer::new()
            .allow_methods([
                axum::http::Method::GET,
                axum::http::Method::POST,
                axum::http::Method::OPTIONS,
            ])
            .allow_headers(Any)
            .allow_origin(Any))
    } else {
        let origins: Vec<axum::http::HeaderValue> = cfg
            .cors_allowed_origins
            .iter()
            .map(|o| {
                o.parse::<axum::http::HeaderValue>()
                    .map_err(|e| ApiError::Internal(format!("invalid CORS_ALLOWED_ORIGINS: {e}")))
            })
            .collect::<Result<_, _>>()?;
        Ok(CorsLayer::new()
            .allow_methods([
                axum::http::Method::GET,
                axum::http::Method::POST,
                axum::http::Method::OPTIONS,
            ])
            .allow_headers(Any)
            .allow_origin(AllowOrigin::list(origins)))
    }
}

async fn graphiql() -> impl IntoResponse {
    axum::response::Html(GraphiQLSource::build().endpoint("/query").finish())
}

async fn graphql_handler(
    Extension(clients): Extension<clients::Clients>,
    Extension(schema): Extension<graphql::AppSchema>,
    identity: Option<Extension<auth::Identity>>,
    req: async_graphql_axum::GraphQLRequest,
) -> async_graphql_axum::GraphQLResponse {
    let mut req = req.into_inner().data(clients);
    if let Some(Extension(identity)) = identity {
        req = req.data(identity);
    }
    schema.execute(req).await.into()
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let cfg = config::Config::from_env()?;

    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::new(&cfg.log_level))
        .init();

    // Redis-пул используется для rate limiter, feature flags и pub/sub подписок.
    let redis_cfg = deadpool_redis::Config::from_url(&cfg.redis_addr);
    let redis_pool = redis_cfg.create_pool(Some(deadpool_redis::Runtime::Tokio1))?;

    let clients = clients::Clients::new(&cfg).await?;
    let redis_client = redis::Client::open(cfg.redis_addr.as_str())?;
    let feature_flags = feature_flags::FlagStore::redis(redis_pool.clone());
    let rate_limiter = ratelimit::RateLimiter::redis(
        redis_pool.clone(),
        cfg.rate_limit_requests,
        cfg.rate_limit_window_seconds,
    );
    let prometheus = metrics::setup()?;

    // В schema.data кладём пул, отдельный Redis-клиент для pub/sub и клиентов.
    let schema = graphql::create_schema(
        redis_pool,
        redis_client,
        clients.clone(),
        cfg.introspection_enabled,
    );
    let verifier = auth::JwtVerifier::new(cfg.jwt_secret.clone());

    let cors = build_cors_layer(&cfg)?;

    let mut app = Router::new();
    if cfg.introspection_enabled {
        app = app.route("/", get(graphiql));
    }
    let app = app
        .route("/query", get(graphql_handler).post(graphql_handler))
        .route("/ws", get(ws::handler))
        .nest("/admin", admin::router(prometheus, feature_flags))
        .layer(from_fn_with_state(verifier, auth::auth_middleware))
        .layer(from_fn_with_state(
            rate_limiter,
            ratelimit::rate_limit_middleware,
        ))
        .layer(from_fn(metrics::track_metrics))
        .layer(cors)
        .layer(Extension(clients))
        .layer(Extension(schema));

    let addr = format!("0.0.0.0:{}", cfg.http_port);
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    tracing::info!("api-gateway listening on http://{}", addr);

    axum::serve(
        listener,
        app.into_make_service_with_connect_info::<SocketAddr>(),
    )
    .await?;
    Ok(())
}
