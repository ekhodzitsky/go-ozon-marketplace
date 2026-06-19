use crate::error::ApiError;
use once_cell::sync::Lazy;
use regex::Regex;

static EMAIL_RE: Lazy<Regex> =
    Lazy::new(|| Regex::new(r"^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$").unwrap());

pub fn validate_email(email: &str) -> Result<(), ApiError> {
    if email.is_empty() || !EMAIL_RE.is_match(email) {
        return Err(ApiError::InvalidArgument(format!("invalid email: {email}")));
    }
    Ok(())
}

pub fn validate_uuid(id: &str) -> Result<(), ApiError> {
    if uuid::Uuid::parse_str(id).is_err() {
        return Err(ApiError::InvalidArgument(format!("invalid uuid: {id}")));
    }
    Ok(())
}

pub fn validate_positive_price(price: f64) -> Result<(), ApiError> {
    if !price.is_finite() || price <= 0.0 {
        return Err(ApiError::InvalidArgument(format!(
            "price must be positive, got {price}"
        )));
    }
    Ok(())
}

pub fn validate_positive_quantity(quantity: i32) -> Result<(), ApiError> {
    if quantity <= 0 {
        return Err(ApiError::InvalidArgument(format!(
            "quantity must be positive, got {quantity}"
        )));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn valid_email() {
        assert!(validate_email("user@example.com").is_ok());
    }

    #[test]
    fn invalid_email() {
        assert!(validate_email("not-an-email").is_err());
    }

    #[test]
    fn valid_uuid() {
        assert!(validate_uuid("550e8400-e29b-41d4-a716-446655440000").is_ok());
    }

    #[test]
    fn invalid_uuid() {
        assert!(validate_uuid("not-uuid").is_err());
    }

    #[test]
    fn positive_price_and_quantity() {
        assert!(validate_positive_price(10.5).is_ok());
        assert!(validate_positive_quantity(3).is_ok());
        assert!(validate_positive_price(0.0).is_err());
        assert!(validate_positive_quantity(0).is_err());
    }
}
