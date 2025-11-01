use bigdecimal::BigDecimal;
use diesel_derive_enum::DbEnum;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, DbEnum)]
#[ExistingTypePath = "crate::schema::sql_types::ProductStatus"]
#[derive(Default)]
pub enum ProductStatus {
    #[db_rename = "PREORDER"]
    Preorder,
    #[db_rename = "IN_STOCK"]
    #[default]
    InStock,
    #[db_rename = "OUT_OF_STOCK"]
    OutOfStock,
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

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Variant {
    pub variant_id: i64,
    pub product_id: i64,
    pub size: Option<String>,
    pub color: Option<String>,
    pub stock_quantity: Option<i32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProductDetail {
    pub product_id: i64,
    pub name: String,
    pub description: Option<String>,
    pub base_price: BigDecimal,
    pub status: ProductStatus,
    pub variants: Vec<Variant>,
}
