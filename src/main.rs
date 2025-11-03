use axum::{Router, routing::get};
use dotenvy::dotenv;
use std::net::SocketAddr;
use tracing::{error, info};

mod api;
mod config;
mod core;
mod schema;
mod utils;

use crate::config::AppConfig;
use crate::utils::db;
use crate::utils::storage::StorageService;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    dotenv().ok();
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        .with_target(false)
        .compact()
        .init();

    let cfg = AppConfig::from_env()?;
    info!(addr = %cfg.server_addr, "Starting server");

    let pool = db::connect_pool(&cfg.database_url).await?;
    if let Err(e) = db::run_migrations(&pool).await {
        error!(error = %e, "Database migrations failed");
        return Err(e);
    }

    let storage_service = StorageService::new(cfg.gcs_bucket_name).await?;
    info!("Connected to Google Cloud Storage");

    let app: Router =
        api::router(&pool, storage_service).route("/", get(|| async { "intania-shop-api" }));

    let addr: SocketAddr = cfg.server_addr.parse()?;
    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
