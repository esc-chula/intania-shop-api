use axum::{
    Router,
    routing::{delete, get, post, put},
};

pub mod errors;
pub mod fairings;
pub mod guards;
pub mod handlers;

use crate::api::errors::handle_404;
use crate::api::fairings::cors;
use crate::api::handlers::{health, products, users};
use crate::core::user::{
    repository::DieselRepo as UserRepository, service::Service as UserService,
};
use crate::utils::db::DBPool;

use std::sync::Arc;

#[derive(Clone)]
pub struct ApiState {
    pub pool: DBPool,
    pub user_service: UserService,
}

pub fn router(pool: DBPool) -> Router {
    let user_repo = UserRepository::new(pool.clone());
    let user_service = UserService::new(Arc::new(user_repo));

    let state = ApiState {
        pool: pool.clone(),
        user_service,
    };

    Router::new()
        .route("/health", get(health::health))
        // Product routes
        .route("/products", get(products::list_products))
        .route("/products", post(products::create_product))
        .route("/products/search", get(products::search_products))
        .route("/products/:id", get(products::get_product))
        .route("/products/:id", put(products::update_product))
        .route("/products/:id", delete(products::delete_product))
        // User routes
        .route("/register", post(users::register))
        .route("/login", post(users::login))
        .with_state(state)
        .layer(cors::layer())
        .fallback(handle_404)
}
