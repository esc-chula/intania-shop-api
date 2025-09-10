use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Cart {
    pub cart_id: i64,
    pub user_id: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CartItem {
    pub item_id: i64,
    pub cart_id: i64,
    pub variant_id: i64,
    pub quantity: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AddToCartRequest {
    pub user_id: i64,
    pub variant_id: i64,
    pub quantity: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AddToCartResponse {
    pub item: CartItem,
    pub message: String,
}
