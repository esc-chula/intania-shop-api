use serde::{Deserialize, Serialize};
use diesel_derive_enum::DbEnum;
use bigdecimal::BigDecimal;

#[derive(Debug, Clone, Serialize, Deserialize, DbEnum)]
#[ExistingTypePath = "crate::schema::sql_types::ProductStatus"]
pub enum ProductStatus {
    #[db_rename = "PREORDER"]
    Preorder,
    #[db_rename = "IN_STOCK"]
    InStock,
    #[db_rename = "OUT_OF_STOCK"]
    OutOfStock,
}

impl Default for ProductStatus {
    fn default() -> Self {
        ProductStatus::InStock
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Product {
    pub product_id: i64,
    pub name: String,
    pub description: Option<String>,
    pub base_price: BigDecimal,
    pub status: ProductStatus,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NewProduct {
    pub name: String,
    pub description: Option<String>,
    pub base_price: BigDecimal,
    pub status: Option<ProductStatus>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UpdateProduct {
    pub name: Option<String>,
    pub description: Option<String>,
    pub base_price: Option<BigDecimal>,
    pub status: Option<ProductStatus>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProductListItem {
    pub product_id: i64,
    pub name: String,
    pub base_price: BigDecimal,
    pub status: ProductStatus,
}

impl From<Product> for ProductListItem {
    fn from(product: Product) -> Self {
        ProductListItem {
            product_id: product.product_id,
            name: product.name,
            base_price: product.base_price,
            status: product.status,
        }
    }
}
