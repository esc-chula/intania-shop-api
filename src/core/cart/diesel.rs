use async_trait::async_trait;
use diesel::prelude::*;
use tracing::error;

use crate::schema::{cart, cart_items, variants};
use crate::utils::db::DBPool;
use crate::utils::errors::{Error, ErrorCode};

use super::entity::CartItem;
use super::repository::CartRepository;

#[derive(Insertable)]
#[diesel(table_name = cart)]
struct NewCartModel {
    pub user_id: i64,
}

#[derive(Queryable, Selectable)]
#[diesel(table_name = cart_items)]
#[diesel(check_for_backend(diesel::pg::Pg))]
struct CartItemModel {
    pub item_id: i64,
    pub cart_id: i64,
    pub variant_id: i64,
    pub quantity: Option<i32>,
}

#[derive(Insertable)]
#[diesel(table_name = cart_items)]
struct NewCartItemModel {
    pub cart_id: i64,
    pub variant_id: i64,
    pub quantity: Option<i32>,
}

pub struct DieselCartRepository {
    pool: DBPool,
}

impl DieselCartRepository {
    pub fn new(pool: DBPool) -> Self {
        Self { pool }
    }
}

impl From<CartItemModel> for CartItem {
    fn from(m: CartItemModel) -> Self {
        CartItem {
            item_id: m.item_id,
            cart_id: m.cart_id,
            variant_id: m.variant_id,
            quantity: m.quantity.unwrap_or(0),
        }
    }
}

#[async_trait]
impl CartRepository for DieselCartRepository {
    async fn get_or_create_cart_id(&self, user_id: i64) -> Result<i64, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        // Try to find existing cart
        if let Ok(existing_id) = cart::table
            .filter(cart::user_id.eq(user_id))
            .select(cart::cart_id)
            .first::<i64>(&mut conn)
        {
            return Ok(existing_id);
        }

        // Create new cart
        let new_cart = NewCartModel { user_id };
        let created_id: i64 = diesel::insert_into(cart::table)
            .values(&new_cart)
            .returning(cart::cart_id)
            .get_result::<i64>(&mut conn)
            .map_err(|e| {
                error!(error = %e, user_id, "Failed to create cart");
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to create cart: {}", e),
                )
            })?;
        Ok(created_id)
    }

    async fn add_or_increment_item(
        &self,
        cart_id_val: i64,
        variant_id_val: i64,
        quantity: i32,
    ) -> Result<CartItem, Error> {
        if quantity <= 0 {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Quantity must be greater than 0",
            ));
        }

        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        // Ensure variant exists
        let _variant_exists: i64 = variants::table
            .filter(variants::variant_id.eq(variant_id_val))
            .select(variants::variant_id)
            .first::<i64>(&mut conn)
            .map_err(|e| if let diesel::result::Error::NotFound = e {
                error!(error = %e, variant_id = variant_id_val, "Variant not found");
                Error::with_message(ErrorCode::ResourceNotFound, "Variant not found")
            } else {
                error!(error = %e, variant_id = variant_id_val, "Database error while checking variant");
                Error::with_message(ErrorCode::DatabaseError, format!("Database error: {}", e))
            })?;

        // Check if item exists
        let existing: Result<CartItemModel, diesel::result::Error> = cart_items::table
            .filter(cart_items::cart_id.eq(cart_id_val))
            .filter(cart_items::variant_id.eq(variant_id_val))
            .select(CartItemModel::as_select())
            .first(&mut conn);

        match existing {
            Ok(current) => {
                let new_qty = current.quantity.unwrap_or(0) + quantity;
                let updated: CartItemModel = diesel::update(
                    cart_items::table.filter(cart_items::item_id.eq(current.item_id)),
                )
                .set(cart_items::quantity.eq(Some(new_qty)))
                .returning(CartItemModel::as_returning())
                .get_result(&mut conn)
                .map_err(|e| {
                    error!(error = %e, item_id = current.item_id, "Failed to update cart item quantity");
                    Error::with_message(
                        ErrorCode::DatabaseError,
                        format!("Failed to update item: {}", e),
                    )
                })?;
                Ok(updated.into())
            }
            Err(diesel::result::Error::NotFound) => {
                // Insert new item
                let new_item = NewCartItemModel {
                    cart_id: cart_id_val,
                    variant_id: variant_id_val,
                    quantity: Some(quantity),
                };
                let created: CartItemModel = diesel::insert_into(cart_items::table)
                    .values(&new_item)
                    .returning(CartItemModel::as_returning())
                    .get_result(&mut conn)
                    .map_err(|e| {
                        error!(error = %e, cart_id = cart_id_val, variant_id = variant_id_val, "Failed to add cart item");
                        Error::with_message(
                            ErrorCode::DatabaseError,
                            format!("Failed to add item: {}", e),
                        )
                    })?;
                Ok(created.into())
            }
            Err(e) => {
                error!(error = %e, cart_id = cart_id_val, variant_id = variant_id_val, "Failed to query cart item");
                Err(Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to query cart item: {}", e),
                ))
            }
        }
    }
}
