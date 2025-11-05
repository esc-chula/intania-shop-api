use axum::{extract::State, http::StatusCode, response::IntoResponse, Json};
use std::sync::Arc;

use crate::api::ApiState;
use crate::core::favorite::{diesel::DieselFavoriteRepository, entity::AddFavoriteRequest, service::FavoriteService};

#[derive(serde::Serialize)]
struct ApiResponse<T> { success: bool, data: T }
#[derive(serde::Serialize)]
struct ApiError { success: bool, error: String }

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
        Ok(resp) => (StatusCode::OK, Json(ApiResponse { success: true, data: resp })).into_response(),
        Err(err) => {
            let status = if err.to_string().contains("not found") { StatusCode::NOT_FOUND } else { StatusCode::BAD_REQUEST };
            (status, Json(ApiError { success: false, error: err.to_string() })).into_response()
        }
    }
}
