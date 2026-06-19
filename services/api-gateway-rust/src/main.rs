mod admin;
mod auth;
mod clients;
mod config;
mod error;
mod graphql;
mod proto;
mod ws;

use async_graphql::http::GraphiQLSource;
use axum::{
    extract::Extension,
    http::Method,
    middleware::from_fn_with_state,
    response::IntoResponse,
    routing::get,
    Router,
};
use tower_http::cors::{Any, CorsLayer};

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

    let clients = clients::Clients::new(&cfg).await?;
    let schema = graphql::create_schema();
    let verifier = auth::JwtVerifier::new(cfg.jwt_secret.clone());

    let cors = CorsLayer::new()
        .allow_methods([Method::GET, Method::POST, Method::OPTIONS])
        .allow_headers(Any)
        .allow_origin(Any);

    let app = Router::new()
        .route("/", get(graphiql))
        .route("/query", get(graphql_handler).post(graphql_handler))
        .route("/ws", get(ws::handler))
        .nest("/admin", admin::router())
        .layer(from_fn_with_state(verifier, auth::auth_middleware))
        .layer(cors)
        .layer(Extension(clients))
        .layer(Extension(schema));

    let addr = format!("0.0.0.0:{}", cfg.http_port);
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    tracing::info!("api-gateway-rust listening on http://{}", addr);

    axum::serve(listener, app).await?;
    Ok(())
}
