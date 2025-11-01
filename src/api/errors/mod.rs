use axum::{
    Json,
    http::StatusCode,
    response::{IntoResponse, Response},
};
use serde_json::json;
use std::fmt;

#[derive(Debug)]
pub enum ApiError {
    NotFound(String),
    BadRequest(String),
    InternalServerError(String),
    Unauthorized(String),
    Forbidden(String),
}

impl fmt::Display for ApiError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ApiError::NotFound(msg) => write!(f, "Not Found: {}", msg),
            ApiError::BadRequest(msg) => write!(f, "Bad Request: {}", msg),
            ApiError::InternalServerError(msg) => write!(f, "Internal Server Error: {}", msg),
            ApiError::Unauthorized(msg) => write!(f, "Unauthorized: {}", msg),
            ApiError::Forbidden(msg) => write!(f, "Forbidden: {}", msg),
        }
    }
}

impl std::error::Error for ApiError {}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, error_type) = match self {
            ApiError::NotFound(_) => (StatusCode::NOT_FOUND, "NOT_FOUND"),
            ApiError::BadRequest(_) => (StatusCode::BAD_REQUEST, "BAD_REQUEST"),
            ApiError::InternalServerError(_) => {
                (StatusCode::INTERNAL_SERVER_ERROR, "INTERNAL_SERVER_ERROR")
            }
            ApiError::Unauthorized(_) => (StatusCode::UNAUTHORIZED, "UNAUTHORIZED"),
            ApiError::Forbidden(_) => (StatusCode::FORBIDDEN, "FORBIDDEN"),
        };

        let body = Json(json!({
            "error": {
                "type": error_type,
                "message": self.to_string()
            }
        }));

        (status, body).into_response()
    }
}

pub fn not_found(message: impl Into<String>) -> ApiError {
    ApiError::NotFound(message.into())
}

#[allow(dead_code)] // Expected to be used as the API grows
pub fn bad_request(message: impl Into<String>) -> ApiError {
    ApiError::BadRequest(message.into())
}

#[allow(dead_code)] // Expected to be used as the API grows
pub fn internal_error(message: impl Into<String>) -> ApiError {
    ApiError::InternalServerError(message.into())
}

#[allow(dead_code)] // Expected to be used as the API grows
pub fn unauthorized(message: impl Into<String>) -> ApiError {
    ApiError::Unauthorized(message.into())
}

#[allow(dead_code)] // Expected to be used as the API grows
pub fn forbidden(message: impl Into<String>) -> ApiError {
    ApiError::Forbidden(message.into())
}

pub async fn handle_404() -> impl IntoResponse {
    not_found("The requested resource was not found")
}

#[allow(dead_code)] // Expected to be used for error handling
pub async fn handle_error(err: Box<dyn std::error::Error + Send + Sync>) -> impl IntoResponse {
    eprintln!("Unhandled error: {}", err);
    internal_error("An unexpected error occurred")
}
