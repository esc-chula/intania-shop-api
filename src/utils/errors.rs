use std::fmt;

// Error codes for application-level errors
#[derive(Debug, Clone, PartialEq)]
pub enum ErrorCode {
    InternalError,
    ValidationError,
    DatabaseError,
    ResourceAlreadyExists,
    ResourceNotFound,
    InvalidCredentials,
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
            ErrorCode::InternalError => "Internal server error",
            ErrorCode::ValidationError => "Validation error",
            ErrorCode::DatabaseError => "Database error",
            ErrorCode::ResourceAlreadyExists => "Resource already exists",
            ErrorCode::ResourceNotFound => "Resource not found",
            ErrorCode::InvalidCredentials => "Invalid credentials",
        }
        .to_string();
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
