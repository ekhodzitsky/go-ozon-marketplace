use crate::graphql::AppSchema;
use async_graphql_axum::{GraphQLProtocol, GraphQLWebSocket};
use axum::{
    extract::{Extension, WebSocketUpgrade},
    response::IntoResponse,
};

/// WebSocket-эндпоинт для GraphQL subscriptions.
/// Поддерживает протоколы `graphql-ws` и `graphql-transport-ws`.
pub async fn handler(
    Extension(schema): Extension<AppSchema>,
    protocol: GraphQLProtocol,
    websocket: WebSocketUpgrade,
) -> impl IntoResponse {
    websocket
        .protocols(["graphql-ws", "graphql-transport-ws"])
        .on_upgrade(move |socket| GraphQLWebSocket::new(socket, schema, protocol).serve())
}
