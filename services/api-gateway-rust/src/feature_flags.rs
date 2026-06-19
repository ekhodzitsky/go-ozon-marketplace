use crate::auth::Identity;
use crate::error::ApiError;
use axum::{
    extract::{Extension, Path, Request},
    http::StatusCode,
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::{get, post},
    Json, Router,
};
use serde::Serialize;
use serde_json::json;
use std::collections::HashMap;
use std::sync::{Arc, Mutex};

/// Хранилище фича-флагов. Redis-реализация использует hash `feature_flags`,
/// memory-реализация — для unit-тестов без Redis.
#[derive(Clone)]
pub enum FlagStore {
    Redis {
        pool: deadpool_redis::Pool,
    },
    Memory {
        map: Arc<Mutex<HashMap<String, bool>>>,
    },
}

impl Default for FlagStore {
    fn default() -> Self {
        Self::memory()
    }
}

impl FlagStore {
    pub fn redis(pool: deadpool_redis::Pool) -> Self {
        Self::Redis { pool }
    }

    pub fn memory() -> Self {
        Self::Memory {
            map: Arc::new(Mutex::new(HashMap::new())),
        }
    }

    pub async fn list(&self) -> Result<Vec<Flag>, ApiError> {
        match self {
            Self::Redis { pool } => {
                let mut conn = pool
                    .get()
                    .await
                    .map_err(|e| ApiError::Internal(e.to_string()))?;
                let map: HashMap<String, bool> = redis::cmd("HGETALL")
                    .arg("feature_flags")
                    .query_async::<HashMap<String, bool>>(&mut conn)
                    .await
                    .map_err(ApiError::from)?;
                Ok(map
                    .into_iter()
                    .map(|(name, enabled)| Flag { name, enabled })
                    .collect())
            }
            Self::Memory { map } => {
                let map = map.lock().unwrap();
                Ok(map
                    .iter()
                    .map(|(name, enabled)| Flag {
                        name: name.clone(),
                        enabled: *enabled,
                    })
                    .collect())
            }
        }
    }

    pub async fn set(&self, name: &str, enabled: bool) -> Result<(), ApiError> {
        match self {
            Self::Redis { pool } => {
                let mut conn = pool
                    .get()
                    .await
                    .map_err(|e| ApiError::Internal(e.to_string()))?;
                redis::cmd("HSET")
                    .arg("feature_flags")
                    .arg(name)
                    .arg(if enabled { 1 } else { 0 })
                    .query_async::<()>(&mut conn)
                    .await
                    .map_err(ApiError::from)?;
                Ok(())
            }
            Self::Memory { map } => {
                map.lock().unwrap().insert(name.to_string(), enabled);
                Ok(())
            }
        }
    }

    #[allow(dead_code)]
    pub async fn is_enabled(&self, name: &str) -> Result<bool, ApiError> {
        match self {
            Self::Redis { pool } => {
                let mut conn = pool
                    .get()
                    .await
                    .map_err(|e| ApiError::Internal(e.to_string()))?;
                let v: Option<i32> = redis::cmd("HGET")
                    .arg("feature_flags")
                    .arg(name)
                    .query_async::<Option<i32>>(&mut conn)
                    .await
                    .map_err(ApiError::from)?;
                Ok(v.unwrap_or(0) != 0)
            }
            Self::Memory { map } => Ok(*map.lock().unwrap().get(name).unwrap_or(&false)),
        }
    }
}

#[derive(Clone, Debug, Serialize)]
pub struct Flag {
    pub name: String,
    pub enabled: bool,
}

pub fn router(store: FlagStore) -> Router {
    Router::new()
        .route("/flags", get(list_flags))
        .route("/flags/:flag/enable", post(enable_flag))
        .route("/flags/:flag/disable", post(disable_flag))
        .route_layer(middleware::from_fn(require_admin))
        .layer(Extension(store))
}

async fn require_admin(
    identity: Option<Extension<Identity>>,
    request: Request,
    next: Next,
) -> Result<Response, ApiError> {
    match identity {
        Some(Extension(id)) if id.is_admin() => Ok(next.run(request).await),
        Some(_) => Err(ApiError::Forbidden),
        None => Err(ApiError::Unauthenticated),
    }
}

async fn list_flags(Extension(store): Extension<FlagStore>) -> Result<Json<Vec<Flag>>, ApiError> {
    let flags = store.list().await?;
    Ok(Json(flags))
}

async fn enable_flag(
    Extension(store): Extension<FlagStore>,
    Path(flag): Path<String>,
) -> Result<impl IntoResponse, ApiError> {
    store.set(&flag, true).await?;
    Ok((
        StatusCode::OK,
        Json(json!({ "status": "enabled", "flag": flag })),
    ))
}

async fn disable_flag(
    Extension(store): Extension<FlagStore>,
    Path(flag): Path<String>,
) -> Result<impl IntoResponse, ApiError> {
    store.set(&flag, false).await?;
    Ok((
        StatusCode::OK,
        Json(json!({ "status": "disabled", "flag": flag })),
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn memory_store_set_and_list() {
        let store = FlagStore::memory();
        store.set("fast-search", true).await.unwrap();
        store.set("new-checkout", false).await.unwrap();
        let flags = store.list().await.unwrap();
        assert_eq!(flags.len(), 2);
        assert!(store.is_enabled("fast-search").await.unwrap());
        assert!(!store.is_enabled("new-checkout").await.unwrap());
    }
}
