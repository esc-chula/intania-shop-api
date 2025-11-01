use axum::{
    Json,
    extract::{Path, Query, State},
    http::StatusCode,
    response::IntoResponse,
};
use std::sync::Arc;

use crate::api::ApiState;
use crate::api::handlers::product::response::{
    ErrorResponse, ProductQuery, ProductResponse, SearchQuery,
};
use crate::core::product::{DieselProductRepository, NewProduct, ProductService, UpdateProduct};
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
        Ok(product) => (StatusCode::CREATED, Json(ProductResponse::new(product))).into_response(),
        Err(e) => {
            let status = if e.to_string().contains("already exists") {
                StatusCode::CONFLICT
            } else {
                StatusCode::BAD_REQUEST
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
        Ok(product) => (StatusCode::OK, Json(ProductResponse::new(product))).into_response(),
        Err(e) => {
            let status = if e.to_string().contains("not found") {
                StatusCode::NOT_FOUND
            } else {
                StatusCode::BAD_REQUEST
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
        Ok(response) => (StatusCode::OK, Json(ProductResponse::new(response))).into_response(),
        Err(e) => (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new(e.to_string())),
        )
            .into_response(),
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
        Ok(product) => (StatusCode::OK, Json(ProductResponse::new(product))).into_response(),
        Err(e) => {
            let status = if e.to_string().contains("not found") {
                StatusCode::NOT_FOUND
            } else {
                StatusCode::BAD_REQUEST
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
        Ok(()) => (StatusCode::NO_CONTENT, "").into_response(),
        Err(e) => {
            let status = if e.to_string().contains("not found") {
                StatusCode::NOT_FOUND
            } else {
                StatusCode::BAD_REQUEST
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
        Ok(products) => (StatusCode::OK, Json(ProductResponse::new(products))).into_response(),
        Err(e) => (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new(e.to_string())),
        )
            .into_response(),
    }
}
