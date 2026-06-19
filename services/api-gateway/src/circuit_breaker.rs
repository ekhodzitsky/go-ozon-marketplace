use crate::error::ApiError;
use std::future::Future;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

/// Простой circuit breaker: после threshold ошибок подряд breaker открывается
/// на timeout. Следующий вызов в half-open режиме — успех закрывает, ошибка открывает снова.
#[derive(Clone, Debug)]
pub struct CircuitBreaker {
    inner: Arc<Mutex<State>>,
}

#[derive(Debug)]
struct State {
    status: Status,
    failures: u32,
    last_failure: Option<Instant>,
    threshold: u32,
    timeout: Duration,
}

#[derive(Debug, Clone, PartialEq)]
enum Status {
    Closed,
    Open,
    HalfOpen,
}

impl CircuitBreaker {
    pub fn new(threshold: u32, timeout: Duration) -> Self {
        Self {
            inner: Arc::new(Mutex::new(State {
                status: Status::Closed,
                failures: 0,
                last_failure: None,
                threshold,
                timeout,
            })),
        }
    }

    /// Проверяет состояние breaker до вызова. Если breaker открыт — возвращает ошибку.
    fn pre_call(&self, service: &str) -> Result<Status, ApiError> {
        let mut state = self.inner.lock().unwrap();
        if let Status::Open = state.status {
            let elapsed = state.last_failure.unwrap_or_else(Instant::now).elapsed();
            if elapsed < state.timeout {
                return Err(ApiError::CircuitOpen {
                    service: service.to_string(),
                });
            }
            state.status = Status::HalfOpen;
        }
        Ok(state.status.clone())
    }

    pub async fn call<F, Fut, T>(&self, service: &str, f: F) -> Result<T, ApiError>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = Result<T, ApiError>>,
    {
        self.pre_call(service)?;
        match f().await {
            Ok(value) => {
                self.record_success();
                Ok(value)
            }
            Err(err) => {
                self.record_failure();
                Err(err)
            }
        }
    }

    fn record_success(&self) {
        let mut state = self.inner.lock().unwrap();
        state.failures = 0;
        state.status = Status::Closed;
    }

    fn record_failure(&self) {
        let mut state = self.inner.lock().unwrap();
        state.failures += 1;
        state.last_failure = Some(Instant::now());
        if state.failures >= state.threshold {
            state.status = Status::Open;
        }
    }
}

impl Default for CircuitBreaker {
    fn default() -> Self {
        // Для dev-окружения: 5 ошибок подряд — открытие на 30 секунд.
        Self::new(5, Duration::from_secs(30))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn stays_closed_on_success() {
        let cb = CircuitBreaker::new(2, Duration::from_secs(1));
        let r = cb.call("svc", || async { Ok::<_, ApiError>(42) }).await;
        assert_eq!(r.unwrap(), 42);
    }

    #[tokio::test]
    async fn opens_after_threshold() {
        let cb = CircuitBreaker::new(2, Duration::from_secs(60));
        let err = || async { Err::<i32, _>(ApiError::Internal("boom".into())) };
        assert!(cb.call("svc", err).await.is_err());
        assert!(cb.call("svc", err).await.is_err());
        // Третий вызов должен вернуть CircuitOpen, а не ошибку downstream.
        let r = cb.call("svc", || async { Ok::<_, ApiError>(1) }).await;
        assert!(matches!(r, Err(ApiError::CircuitOpen { .. })));
    }

    #[tokio::test]
    async fn half_open_closes_on_success() {
        let cb = CircuitBreaker::new(1, Duration::from_millis(10));
        let err = || async { Err::<i32, _>(ApiError::Internal("boom".into())) };
        assert!(cb.call("svc", err).await.is_err());
        tokio::time::sleep(Duration::from_millis(20)).await;
        let r = cb.call("svc", || async { Ok::<_, ApiError>(42) }).await;
        assert_eq!(r.unwrap(), 42);
        // После закрытия снова можно звонить.
        let r2 = cb.call("svc", || async { Ok::<_, ApiError>(43) }).await;
        assert_eq!(r2.unwrap(), 43);
    }
}
