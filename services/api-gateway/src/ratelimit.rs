use crate::auth::Identity;
use crate::error::ApiError;
use axum::{
    extract::{ConnectInfo, Request, State},
    middleware::Next,
    response::Response,
};
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

/// Redis-backed sliding window rate limiter с in-memory fallback для тестов.
#[derive(Clone)]
#[allow(dead_code)]
pub enum RateLimiter {
    Redis {
        pool: deadpool_redis::Pool,
        max: u32,
        window: Duration,
    },
    Memory {
        state: Arc<Mutex<HashMap<String, Vec<Instant>>>>,
        max: u32,
        window: Duration,
    },
}

impl RateLimiter {
    pub fn redis(pool: deadpool_redis::Pool, max: u32, window_seconds: u64) -> Self {
        Self::Redis {
            pool,
            max,
            window: Duration::from_secs(window_seconds),
        }
    }

    #[cfg(test)]
    pub fn memory(max: u32, window_seconds: u64) -> Self {
        Self::Memory {
            state: Arc::new(Mutex::new(HashMap::new())),
            max,
            window: Duration::from_secs(window_seconds),
        }
    }

    /// Проверяет, не превышен ли лимит для ключа. Ключ формируется вызывающим кодом.
    pub async fn check(&self, key: &str) -> Result<(), ApiError> {
        match self {
            Self::Redis { pool, max, window } => check_redis(pool, key, *max, *window).await,
            Self::Memory { state, max, window } => check_memory(state, key, *max, *window),
        }
    }
}

fn check_memory(
    state: &Arc<Mutex<HashMap<String, Vec<Instant>>>>,
    key: &str,
    max: u32,
    window: Duration,
) -> Result<(), ApiError> {
    let mut map = state.lock().unwrap();
    let now = Instant::now();
    let cutoff = now - window;
    let entries = map.entry(key.to_string()).or_default();
    entries.retain(|t| *t >= cutoff);
    if entries.len() >= max as usize {
        return Err(ApiError::RateLimited);
    }
    entries.push(now);
    Ok(())
}

async fn check_redis(
    pool: &deadpool_redis::Pool,
    key: &str,
    max: u32,
    window: Duration,
) -> Result<(), ApiError> {
    let mut conn = pool
        .get()
        .await
        .map_err(|e| ApiError::Internal(e.to_string()))?;
    let redis_key = format!("ratelimit:{key}");
    let now_ms = start_time_ms()?;
    let window_ms = window.as_millis() as u64;
    let cutoff = now_ms.saturating_sub(window_ms);

    // Убираем старые события и считаем оставшиеся в одном pipeline.
    let _: (u64, u64) = redis::pipe()
        .atomic()
        .zrembyscore(&redis_key, 0_i64, cutoff as i64)
        .zcard(&redis_key)
        .query_async(&mut conn)
        .await
        .map_err(ApiError::from)?;

    // Снова берём соединение и проверяем текущее количество.
    let mut conn = pool
        .get()
        .await
        .map_err(|e| ApiError::Internal(e.to_string()))?;
    let count: u64 = redis::cmd("ZCARD")
        .arg(&redis_key)
        .query_async(&mut conn)
        .await
        .map_err(ApiError::from)?;

    if count >= max as u64 {
        return Err(ApiError::RateLimited);
    }

    // Добавляем текущее событие.
    let mut conn = pool
        .get()
        .await
        .map_err(|e| ApiError::Internal(e.to_string()))?;
    redis::pipe()
        .atomic()
        .zadd(&redis_key, now_ms as i64, now_ms as f64)
        .expire(&redis_key, window.as_secs() as i64 + 1)
        .query_async::<()>(&mut conn)
        .await
        .map_err(ApiError::from)?;

    Ok(())
}

fn start_time_ms() -> Result<u64, ApiError> {
    use std::time::SystemTime;
    SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .map(|d| d.as_millis() as u64)
        .map_err(|e| ApiError::Internal(e.to_string()))
}

/// Middleware: лимитирует по IP и, если известен пользователь, по user_id.
pub async fn rate_limit_middleware(
    State(limiter): State<RateLimiter>,
    ConnectInfo(addr): ConnectInfo<SocketAddr>,
    identity: Option<axum::extract::Extension<Identity>>,
    request: Request,
    next: Next,
) -> Result<Response, ApiError> {
    limiter.check(&format!("ip:{}", addr.ip())).await?;
    if let Some(axum::extract::Extension(id)) = identity {
        limiter.check(&format!("user:{}", id.user_id)).await?;
    }
    Ok(next.run(request).await)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn memory_allows_under_limit() {
        let rl = RateLimiter::memory(2, 60);
        assert!(rl.check("k").await.is_ok());
        assert!(rl.check("k").await.is_ok());
    }

    #[tokio::test]
    async fn memory_blocks_over_limit() {
        let rl = RateLimiter::memory(1, 60);
        assert!(rl.check("k").await.is_ok());
        assert!(matches!(rl.check("k").await, Err(ApiError::RateLimited)));
    }

    #[tokio::test]
    async fn memory_window_slides() {
        let rl = RateLimiter::memory(1, 1);
        assert!(rl.check("k").await.is_ok());
        tokio::time::sleep(Duration::from_secs(2)).await;
        assert!(rl.check("k").await.is_ok());
    }
}
