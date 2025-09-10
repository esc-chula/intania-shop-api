use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::IntoResponse,
    Json,
};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

use crate::api::ApiState;
use crate::core::product::{ProductService, NewProduct, UpdateProduct, DieselProductRepository};

#[derive(Debug, Deserialize)]
pub struct ProductQuery {
    page: Option<u32>,
    page_size: Option<u32>,
}

#[derive(Debug, Deserialize)]
pub struct SearchQuery {
    q: String,
    page: Option<u32>,
    page_size: Option<u32>,
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
    fn new(data: T) -> Self {
        Self {
            success: true,
            data,
        }
    }
}

impl ErrorResponse {
    fn new(error: String) -> Self {
        Self {
            success: false,
            error,
        }
    }
}

fn get_product_service(state: &ApiState) -> ProductService {
    let repository = Arc::new(DieselProductRepository::new(state.pool.clone()));
    ProductService::new(repository)
}

// POST /products
pub async fn create_product(
    State(state): State<ApiState>,
    Json(new_product): Json<NewProduct>,
) -> impl IntoResponse {
    let service = get_product_service(&state);

    match service.create_product(new_product).await {
        Ok(product) => {
            (StatusCode::CREATED, Json(ProductResponse::new(product))).into_response()
        }
        Err(e) => {
            let status = match e.to_string().contains("already exists") {
                true => StatusCode::CONFLICT,
                false => StatusCode::BAD_REQUEST,
            };
            (status, Json(ErrorResponse::new(e.to_string()))).into_response()
        }
    }
}

// GET /products/:id
pub async fn get_product(
    State(state): State<ApiState>,
    Path(product_id): Path<i64>,
) -> impl IntoResponse {
    let service = get_product_service(&state);

    match service.get_product(product_id).await {
        Ok(product) => {
            (StatusCode::OK, Json(ProductResponse::new(product))).into_response()
        }
        Err(e) => {
            let status = match e.to_string().contains("not found") {
                true => StatusCode::NOT_FOUND,
                false => StatusCode::BAD_REQUEST,
            };
            (status, Json(ErrorResponse::new(e.to_string()))).into_response()
        }
    }
}

// GET /products
pub async fn list_products(
    State(state): State<ApiState>,
    Query(query): Query<ProductQuery>,
) -> impl IntoResponse {
    let service = get_product_service(&state);

    let page = query.page.unwrap_or(1).max(1);
    let page_size = query.page_size.unwrap_or(10).clamp(1, 100);

    match service.list_products(page, page_size).await {
        Ok(response) => {
            (StatusCode::OK, Json(ProductResponse::new(response))).into_response()
        }
        Err(e) => {
            (StatusCode::BAD_REQUEST, Json(ErrorResponse::new(e.to_string()))).into_response()
        }
    }
}

// PUT /products/:id
pub async fn update_product(
    State(state): State<ApiState>,
    Path(product_id): Path<i64>,
    Json(update_product): Json<UpdateProduct>,
) -> impl IntoResponse {
    let service = get_product_service(&state);

    match service.update_product(product_id, update_product).await {
        Ok(product) => {
            (StatusCode::OK, Json(ProductResponse::new(product))).into_response()
        }
        Err(e) => {
            let status = match e.to_string().contains("not found") {
                true => StatusCode::NOT_FOUND,
                false => StatusCode::BAD_REQUEST,
            };
            (status, Json(ErrorResponse::new(e.to_string()))).into_response()
        }
    }
}

// DELETE /products/:id
pub async fn delete_product(
    State(state): State<ApiState>,
    Path(product_id): Path<i64>,
) -> impl IntoResponse {
    let service = get_product_service(&state);

    match service.delete_product(product_id).await {
        Ok(()) => {
            (StatusCode::NO_CONTENT, "").into_response()
        }
        Err(e) => {
            let status = match e.to_string().contains("not found") {
                true => StatusCode::NOT_FOUND,
                false => StatusCode::BAD_REQUEST,
            };
            (status, Json(ErrorResponse::new(e.to_string()))).into_response()
        }
    }
}

// GET /products/search?q=name
pub async fn search_products(
    State(state): State<ApiState>,
    Query(query): Query<SearchQuery>,
) -> impl IntoResponse {
    let service = get_product_service(&state);

    let page = query.page.unwrap_or(1).max(1);
    let page_size = query.page_size.unwrap_or(10).clamp(1, 100);

    match service.search_products(&query.q, page, page_size).await {
        Ok(products) => {
            (StatusCode::OK, Json(ProductResponse::new(products))).into_response()
        }
        Err(e) => {
            (StatusCode::BAD_REQUEST, Json(ErrorResponse::new(e.to_string()))).into_response()
        }
    }
}
