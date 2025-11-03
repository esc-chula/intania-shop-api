use std::env;

pub struct AppConfig {
    pub server_addr: String,
    pub database_url: String,
    pub gcs_bucket_name: String,
}

impl AppConfig {
    pub fn from_env() -> anyhow::Result<Self> {
        let server_addr = env::var("SERVER_ADDR").unwrap_or_else(|_| "0.0.0.0:8080".to_string());
        let database_url = env::var("DATABASE_URL")
            .map_err(|_| anyhow::anyhow!("Missing env var DATABASE_URL"))?;
        let gcs_bucket_name = env::var("GCS_BUCKET_NAME")
            .map_err(|_| anyhow::anyhow!("Missing env var GCS_BUCKET_NAME"))?;

        Ok(Self {
            server_addr,
            database_url,
            gcs_bucket_name,
        })
    }
}
