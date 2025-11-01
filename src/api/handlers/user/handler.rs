use axum::{
    extract::State,
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde_json::json;

use crate::api::ApiState;
use crate::core::user::entity::{LoginRequest, UserRegistration};
use crate::utils::errors::ErrorCode;

pub async fn register(
    State(state): State<ApiState>,
    Json(registration): Json<UserRegistration>,
) -> impl IntoResponse {
    match state.user_service.register(registration).await {
        Ok(response) => (StatusCode::CREATED, Json(json!(response))),
        Err(err) => {
            let status = match err.code {
                ErrorCode::ValidationError => StatusCode::BAD_REQUEST,
                ErrorCode::ResourceAlreadyExists => StatusCode::CONFLICT,
                _ => StatusCode::INTERNAL_SERVER_ERROR,
            };
            (
                status,
                Json(json!({
                    "error": err.message
                })),
            )
        }
    }
}

pub async fn login(
    State(state): State<ApiState>,
    Json(login_request): Json<LoginRequest>,
) -> impl IntoResponse {
    match state.user_service.login(login_request).await {
        Ok(response) => (StatusCode::OK, Json(json!(response))),
        Err(err) => {
            let status = match err.code {
                ErrorCode::ValidationError => StatusCode::BAD_REQUEST,
                ErrorCode::InvalidCredentials => StatusCode::UNAUTHORIZED,
                _ => StatusCode::INTERNAL_SERVER_ERROR,
            };
            (
                status,
                Json(json!({
                    "error": err.message
                })),
            )
        }
    }
}
