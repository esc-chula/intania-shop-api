use std::sync::Arc;

use crate::utils::errors::{Error, ErrorCode};

use super::entity::{AddToCartRequest, AddToCartResponse, CartItem};
use super::repository::CartRepository;

pub struct CartService {
    repo: Arc<dyn CartRepository>,
}

impl CartService {
    pub fn new(repo: Arc<dyn CartRepository>) -> Self { Self { repo } }

    pub async fn add_to_cart(&self, req: AddToCartRequest) -> Result<AddToCartResponse, Error> {
        if req.quantity <= 0 {
            return Err(Error::with_message(ErrorCode::ValidationError, "Quantity must be greater than 0"));
        }
        let cart_id = self.repo.get_or_create_cart_id(req.user_id).await?;
        let item: CartItem = self.repo.add_or_increment_item(cart_id, req.variant_id, req.quantity).await?;
        Ok(AddToCartResponse { item, message: "Item added to cart".to_string() })
    }
}
