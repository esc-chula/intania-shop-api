use serde::Serialize;
use std::fmt;

// Error codes for application-level errors
#[derive(Debug, Clone, PartialEq)]
pub enum ErrorCode {
    NotAuthorized,
    NotFound,
    BadRequest,
    InternalError,
    ValidationError,
    DatabaseError,
    ResourceAlreadyExists,
    ResourceNotFound,
    InvalidCredentials,
    Unknown,
}

// Application-level error type (for business logic)
#[derive(Debug)]
pub struct Error {
    pub code: ErrorCode,
    pub message: String,
}

impl Error {
    pub fn new(code: ErrorCode) -> Self {
        let message = match code {
            ErrorCode::NotAuthorized => "Not authorized",
            ErrorCode::NotFound => "Resource not found",
            ErrorCode::BadRequest => "Bad request",
            ErrorCode::InternalError => "Internal server error",
            ErrorCode::ValidationError => "Validation error",
            ErrorCode::DatabaseError => "Database error",
            ErrorCode::ResourceAlreadyExists => "Resource already exists",
            ErrorCode::ResourceNotFound => "Resource not found",
            ErrorCode::InvalidCredentials => "Invalid credentials",
            ErrorCode::Unknown => "Unknown error",
        }.to_string();
        Self { code, message }
    }

    pub fn with_message(code: ErrorCode, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
        }
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for Error {}

impl From<anyhow::Error> for Error {
    fn from(err: anyhow::Error) -> Self {
        Self::with_message(ErrorCode::InternalError, err.to_string())
    }
}

#[derive(Debug, Serialize)]
pub struct Success {
    pub success: bool,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<serde_json::Value>,
}

impl Success {
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            success: true,
            message: message.into(),
            data: None,
        }
    }

    pub fn with_data(message: impl Into<String>, data: serde_json::Value) -> Self {
        Self {
            success: true,
            message: message.into(),
            data: Some(data),
        }
    }
}