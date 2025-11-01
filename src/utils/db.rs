use diesel::pg::PgConnection;
use diesel::r2d2::{self, ConnectionManager};
use diesel_migrations::{EmbeddedMigrations, MigrationHarness};

pub type Pool = r2d2::Pool<ConnectionManager<PgConnection>>;
pub type DBPool = r2d2::Pool<ConnectionManager<PgConnection>>;

pub const MIGRATIONS: EmbeddedMigrations = diesel_migrations::embed_migrations!("migrations");

pub async fn connect_pool(database_url: &str) -> anyhow::Result<DBPool> {
    let manager = ConnectionManager::<PgConnection>::new(database_url);
    let pool = r2d2::Pool::builder().max_size(10).build(manager)?;
    Ok(pool)
}

pub fn get_connection(
    pool: &DBPool,
) -> anyhow::Result<r2d2::PooledConnection<ConnectionManager<PgConnection>>> {
    pool.get()
        .map_err(|e| anyhow::anyhow!("Failed to get database connection: {}", e))
}

pub async fn run_migrations(pool: &DBPool) -> anyhow::Result<()> {
    let pool = pool.clone();
    tokio::task::spawn_blocking(move || -> anyhow::Result<()> {
        let mut conn = pool.get()?;
        conn.run_pending_migrations(MIGRATIONS)
            .map(|_| ())
            .map_err(|e| anyhow::anyhow!(e))
    })
    .await??;
    Ok(())
}
