use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
    Json,
};
use serde_json::json;
use thiserror::Error;

/// Базовые ошибки API-gateway. Реализуют HTTP-ответ и GraphQL-ошибку.
#[derive(Error, Debug, Clone)]
pub enum ApiError {
    #[error("unauthenticated")]
    Unauthenticated,
    #[error("forbidden")]
    Forbidden,
    #[error("downstream error: {0}")]
    Downstream(String),
    #[error("invalid argument: {0}")]
    InvalidArgument(String),
    #[error("internal error: {0}")]
    Internal(String),
}

impl ApiError {
    pub fn status(&self) -> StatusCode {
        match self {
            Self::Unauthenticated => StatusCode::UNAUTHORIZED,
            Self::Forbidden => StatusCode::FORBIDDEN,
            Self::InvalidArgument(_) => StatusCode::BAD_REQUEST,
            Self::Downstream(_) | Self::Internal(_) => StatusCode::INTERNAL_SERVER_ERROR,
        }
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let body = Json(json!({ "error": self.to_string() }));
        (self.status(), body).into_response()
    }
}

impl From<tonic::Status> for ApiError {
    fn from(status: tonic::Status) -> Self {
        Self::Downstream(status.message().to_string())
    }
}

impl From<jsonwebtoken::errors::Error> for ApiError {
    fn from(_err: jsonwebtoken::errors::Error) -> Self {
        Self::Unauthenticated
    }
}

impl From<std::env::VarError> for ApiError {
    fn from(err: std::env::VarError) -> Self {
        Self::Internal(err.to_string())
    }
}
