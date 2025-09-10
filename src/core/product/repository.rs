use async_trait::async_trait;
use crate::utils::errors::Error;
use super::entity::{Product, NewProduct, UpdateProduct, ProductListItem};

#[async_trait]
pub trait ProductRepository: Send + Sync {
    async fn create(&self, new_product: NewProduct) -> Result<Product, Error>;
    async fn find_by_id(&self, product_id: i64) -> Result<Product, Error>;
    async fn find_all(&self, offset: i64, limit: i64) -> Result<Vec<ProductListItem>, Error>;
    async fn update(&self, product_id: i64, update_product: UpdateProduct) -> Result<Product, Error>;
    async fn delete(&self, product_id: i64) -> Result<(), Error>;
    async fn search_by_name(&self, name: &str, offset: i64, limit: i64) -> Result<Vec<ProductListItem>, Error>;
    async fn count_total(&self) -> Result<i64, Error>;
}
