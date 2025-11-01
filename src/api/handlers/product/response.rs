use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize)]
pub struct ProductQuery {
    pub page: Option<u32>,
    pub page_size: Option<u32>,
}

#[derive(Debug, Deserialize)]
pub struct SearchQuery {
    pub q: String,
    pub page: Option<u32>,
    pub page_size: Option<u32>,
}

#[derive(Debug, Serialize)]
pub struct ProductResponse<T> {
    success: bool,
    data: T,
}

#[derive(Debug, Serialize)]
pub struct ErrorResponse {
    success: bool,
    error: String,
}

impl<T> ProductResponse<T> {
    pub fn new(data: T) -> Self {
        Self {
            success: true,
            data,
        }
    }
}

impl ErrorResponse {
    pub fn new(error: String) -> Self {
        Self {
            success: false,
            error,
        }
    }
}
