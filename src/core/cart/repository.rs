use async_trait::async_trait;

use crate::utils::errors::Error;

use super::entity::{CartItem};

#[async_trait]
pub trait CartRepository: Send + Sync {
    async fn get_or_create_cart_id(&self, user_id: i64) -> Result<i64, Error>;
    async fn add_or_increment_item(&self, cart_id: i64, variant_id: i64, quantity: i32) -> Result<CartItem, Error>;
}
