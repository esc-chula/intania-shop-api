use axum::{Json, extract::State, http::StatusCode, response::IntoResponse};
use std::sync::Arc;

use crate::api::ApiState;
use crate::api::response::{ApiError, ApiResponse};
use crate::core::cart::{
    diesel::DieselCartRepository, entity::AddToCartRequest, service::CartService,
};

fn get_service(state: &ApiState) -> CartService {
    let repo = Arc::new(DieselCartRepository::new(state.pool.clone()));
    CartService::new(repo)
}

// PUT /cart/items
pub async fn add_item(
    State(state): State<ApiState>,
    Json(req): Json<AddToCartRequest>,
) -> impl IntoResponse {
    let service = get_service(&state);
    match service.add_to_cart(req).await {
        Ok(resp) => (StatusCode::OK, Json(ApiResponse::ok(resp))).into_response(),
        Err(err) => (
            StatusCode::BAD_REQUEST,
            Json(ApiError::new(err.to_string())),
        )
            .into_response(),
    }
}
