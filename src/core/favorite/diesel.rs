use async_trait::async_trait;
use diesel::prelude::*;
use tracing::error;

use crate::schema::{favorites, products};
use crate::utils::db::DBPool;
use crate::utils::errors::{Error, ErrorCode};

use super::entity::{AddFavoriteRequest, Favorite};
use super::repository::FavoriteRepository;

#[derive(Queryable, Selectable)]
#[diesel(table_name = favorites)]
#[diesel(check_for_backend(diesel::pg::Pg))]
struct FavoriteModel {
    pub user_id: i64,
    pub product_id: i64,
    pub created_at: chrono::NaiveDateTime,
}

#[derive(Insertable)]
#[diesel(table_name = favorites)]
struct NewFavoriteModel {
    pub user_id: i64,
    pub product_id: i64,
}

impl From<FavoriteModel> for Favorite {
    fn from(m: FavoriteModel) -> Self {
        Favorite {
            user_id: m.user_id,
            product_id: m.product_id,
            created_at: m.created_at,
        }
    }
}

pub struct DieselFavoriteRepository {
    pool: DBPool,
}

impl DieselFavoriteRepository {
    pub fn new(pool: DBPool) -> Self {
        Self { pool }
    }
}

#[async_trait]
impl FavoriteRepository for DieselFavoriteRepository {
    async fn add(&self, req: AddFavoriteRequest) -> Result<Favorite, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            error!(error = %e, user_id = req.user_id, product_id = req.product_id, "DB connection error while adding favorite");
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        // verify product exists
        let _product_exists: i64 = products::table
            .filter(products::id.eq(req.product_id))
            .select(products::id)
            .first::<i64>(&mut conn)
            .map_err(|e| if let diesel::result::Error::NotFound = e {
                error!(error = %e, product_id = req.product_id, "Product not found when adding favorite");
                Error::with_message(ErrorCode::ResourceNotFound, "Product not found")
            } else {
                error!(error = %e, product_id = req.product_id, "Database error while verifying product");
                Error::with_message(ErrorCode::DatabaseError, format!("Database error: {}", e))
            })?;

        // insert if not exists (idempotent)
        let new_row = NewFavoriteModel {
            user_id: req.user_id,
            product_id: req.product_id,
        };

        // Try insert; on conflict do nothing
        let _ = diesel::insert_into(favorites::table)
            .values(&new_row)
            .on_conflict((favorites::user_id, favorites::product_id))
            .do_nothing()
            .execute(&mut conn)
            .map_err(|e| {
                error!(error = %e, user_id = req.user_id, product_id = req.product_id, "Failed to add favorite");
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to add favorite: {}", e),
                )
            })?;

        // Read back the row
        let fav: FavoriteModel = favorites::table
            .filter(favorites::user_id.eq(req.user_id))
            .filter(favorites::product_id.eq(req.product_id))
            .select(FavoriteModel::as_select())
            .first(&mut conn)
            .map_err(|e| {
                error!(error = %e, user_id = req.user_id, product_id = req.product_id, "Failed to fetch favorite after insert");
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to fetch favorite: {}", e),
                )
            })?;

        Ok(fav.into())
    }
}
