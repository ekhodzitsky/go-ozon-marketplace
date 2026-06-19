use axum::{
    extract::ws::WebSocketUpgrade,
    response::IntoResponse,
};

/// Заглушка WebSocket-эндпоинта. Реальная логика будет добавлена позже.
pub async fn handler(ws: WebSocketUpgrade) -> impl IntoResponse {
    ws.on_upgrade(|_socket| async move {
        // TODO: подписки на orderStatusChanged / inventoryChanged.
    })
}
