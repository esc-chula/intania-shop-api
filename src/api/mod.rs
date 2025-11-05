use axum::{
    Router,
    extract::DefaultBodyLimit,
    routing::{delete, get, post, put},
};

pub mod errors;
pub mod fairings;
pub mod guards;
pub mod handlers;

use crate::api::errors::handle_404;
use crate::api::fairings::cors;
use crate::api::handlers::{
    health, product::handler as product_handler, upload, user::handler as user_handler, cart as cart_handler, favorite as favorite_handler,
};
use crate::core::user::{
    repository::DieselRepo as UserRepository, service::Service as UserService,
};
use crate::utils::db::DBPool;
use crate::utils::storage::StorageService;

use std::sync::Arc;

#[derive(Clone)]
pub struct ApiState {
    pub pool: DBPool,
    pub user_service: UserService,
    pub storage_service: StorageService,
}

pub fn router(pool: &DBPool, storage_service: StorageService) -> Router {
    let user_repo = UserRepository::new(pool.clone());
    let user_service = UserService::new(Arc::new(user_repo));

    let state = ApiState {
        pool: pool.clone(),
        user_service,
        storage_service,
    };

    Router::new()
        .route("/health", get(health::health))
        .nest(
            "/products",
            Router::new()
                .route("/", get(product_handler::list_products))
                .route("/", post(product_handler::create_product))
                .route("/search", get(product_handler::search_products))
                .route("/:id", get(product_handler::get_product))
                .route("/:id", put(product_handler::update_product))
                .route("/:id", delete(product_handler::delete_product)),
        )
        .nest(
            "/upload",
            Router::new()
                .route("/product-images", post(upload::upload_product_images))
                .route("/product-videos", post(upload::upload_product_videos))
                .layer(DefaultBodyLimit::max(100 * 1024 * 1024)), // 100MB limit for uploads
        )
        .nest(
            "/auth",
            Router::new()
                .route("/register", post(user_handler::register))
                .route("/login", post(user_handler::login)),
        )
        .nest(
            "/cart",
            Router::new()
                .route("/items", put(cart_handler::add_item)),
        )
        .nest(
            "/favorites",
            Router::new()
                .route("/", put(favorite_handler::add_favorite)),
        )
        .with_state(state)
        .layer(cors::layer())
        .fallback(handle_404)
}
