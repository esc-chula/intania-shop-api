use axum::{Json, extract::State, http::StatusCode, response::IntoResponse};
use std::sync::Arc;

use crate::api::ApiState;
use crate::api::response::{ApiError, ApiResponse};
use crate::core::favorite::{
    diesel::DieselFavoriteRepository, entity::AddFavoriteRequest, service::FavoriteService,
};

fn get_service(state: &ApiState) -> FavoriteService {
    let repo = Arc::new(DieselFavoriteRepository::new(state.pool.clone()));
    FavoriteService::new(repo)
}

// PUT /favorites
pub async fn add_favorite(
    State(state): State<ApiState>,
    Json(req): Json<AddFavoriteRequest>,
) -> impl IntoResponse {
    let service = get_service(&state);
    match service.add(req).await {
        Ok(resp) => (StatusCode::OK, Json(ApiResponse::ok(resp))).into_response(),
        Err(err) => {
            let status = if err.to_string().contains("not found") {
                StatusCode::NOT_FOUND
            } else {
                StatusCode::BAD_REQUEST
            };
            (status, Json(ApiError::new(err.to_string()))).into_response()
        }
    }
}
