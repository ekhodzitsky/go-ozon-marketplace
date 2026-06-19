use crate::error::ApiError;
use async_graphql::Context;
use axum::{
    extract::{Request, State},
    http::header::AUTHORIZATION,
    middleware::Next,
    response::Response,
};
use jsonwebtoken::{decode, Algorithm, DecodingKey, Validation};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Claims {
    pub sub: String,
    pub role: String,
    pub exp: usize,
}

/// Идентификация вызывающего пользователя, которую middleware кладёт в request extensions.
#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct Identity {
    pub user_id: String,
    pub role: String,
    pub token: String,
}

impl Identity {
    pub fn is_admin(&self) -> bool {
        self.role == "admin"
    }
}

/// Верификатор JWT. Создаётся один раз и передаётся как axum State.
#[derive(Clone)]
pub struct JwtVerifier {
    secret: String,
}

impl JwtVerifier {
    pub fn new(secret: impl Into<String>) -> Self {
        Self {
            secret: secret.into(),
        }
    }

    pub fn verify(&self, token: &str) -> Result<Identity, ApiError> {
        let key = DecodingKey::from_secret(self.secret.as_bytes());
        let mut validation = Validation::new(Algorithm::HS256);
        validation.validate_exp = false;
        let token_data = decode::<Claims>(token, &key, &validation)?;
        Ok(Identity {
            user_id: token_data.claims.sub,
            role: token_data.claims.role,
            token: token.to_string(),
        })
    }
}

/// Извлекает токен из Authorization, валидирует его и кладёт Identity в extensions.
pub async fn auth_middleware(
    State(verifier): State<JwtVerifier>,
    mut req: Request,
    next: Next,
) -> Result<Response, ApiError> {
    if let Some(auth_header) = req.headers().get(AUTHORIZATION) {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                let identity = verifier.verify(token)?;
                req.extensions_mut().insert(identity);
            }
        }
    }
    Ok(next.run(req).await)
}

pub fn identity_from_ctx<'a>(ctx: &'a Context<'a>) -> Option<&'a Identity> {
    ctx.data::<Identity>().ok()
}

pub fn require_identity<'a>(ctx: &'a Context<'a>) -> Result<&'a Identity, ApiError> {
    identity_from_ctx(ctx).ok_or(ApiError::Unauthenticated)
}

#[cfg(test)]
mod tests {
    use super::*;
    use jsonwebtoken::{encode, EncodingKey, Header};

    #[test]
    fn verifies_valid_token() {
        let secret = "test-secret";
        let claims = Claims {
            sub: "user-42".into(),
            role: "user".into(),
            exp: usize::MAX,
        };
        let token = encode(&Header::default(), &claims, &EncodingKey::from_secret(secret.as_bytes())).unwrap();

        let verifier = JwtVerifier::new(secret);
        let identity = verifier.verify(&token).unwrap();
        assert_eq!(identity.user_id, "user-42");
        assert_eq!(identity.role, "user");
    }

    #[test]
    fn rejects_bad_signature() {
        let secret = "test-secret";
        let claims = Claims {
            sub: "user-42".into(),
            role: "user".into(),
            exp: usize::MAX,
        };
        let token = encode(&Header::default(), &claims, &EncodingKey::from_secret("other-secret".as_bytes())).unwrap();

        let verifier = JwtVerifier::new(secret);
        assert!(verifier.verify(&token).is_err());
    }
}
