use axum::{routing::{get}, Router};

pub mod guards;
pub mod fairings;
pub mod errors;
pub mod handlers;

use crate::api::errors::handle_404;
use crate::utils::db::DBPool;
use crate::api::fairings::cors;
use crate::api::handlers::health;

#[derive(Clone)]
pub struct ApiState {
    pub pool: DBPool,
}

pub fn router(pool: DBPool) -> Router {
    let state = ApiState { pool };
    Router::new()
        .route("/health", get(health::health))
        .with_state(state)
        .layer(cors::layer())
        .fallback(handle_404)
}

