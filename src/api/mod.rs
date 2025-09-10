use axum::{routing::{get, post, put, delete}, Router};

pub mod guards;
pub mod fairings;
pub mod errors;
pub mod handlers;

use crate::api::errors::handle_404;
use crate::utils::db::DBPool;
use crate::api::fairings::cors;
use crate::api::handlers::{health, products};

#[derive(Clone)]
pub struct ApiState {
    pub pool: DBPool,
}

pub fn router(pool: DBPool) -> Router {
    let state = ApiState { pool };
    Router::new()
        .route("/health", get(health::health))
        // Product routes
        .route("/products", get(products::list_products))
        .route("/products", post(products::create_product))
        .route("/products/search", get(products::search_products))
        .route("/products/:id", get(products::get_product))
        .route("/products/:id", put(products::update_product))
        .route("/products/:id", delete(products::delete_product))
        .with_state(state)
        .layer(cors::layer())
        .fallback(handle_404)
}

