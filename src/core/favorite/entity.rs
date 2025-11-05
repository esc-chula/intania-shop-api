use chrono::NaiveDateTime;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FavoriteKey {
    pub user_id: i64,
    pub product_id: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Favorite {
    pub user_id: i64,
    pub product_id: i64,
    pub created_at: NaiveDateTime,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AddFavoriteRequest {
    pub user_id: i64,
    pub product_id: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AddFavoriteResponse {
    pub user_id: i64,
    pub product_id: i64,
    pub created_at: NaiveDateTime,
    pub message: String,
}
