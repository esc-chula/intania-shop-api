use crate::utils::errors::{Error, ErrorCode};
use super::entity::{Product, NewProduct, UpdateProduct, ProductListItem, ProductStatus};
use super::repository::ProductRepository;
use std::sync::Arc;
use bigdecimal::BigDecimal;

#[derive(Clone)]
pub struct ProductService {
    repository: Arc<dyn ProductRepository>,
}

impl ProductService {
    pub fn new(repository: Arc<dyn ProductRepository>) -> Self {
        Self { repository }
    }

    pub async fn create_product(&self, mut new_product: NewProduct) -> Result<Product, Error> {
        // Business validation
        self.validate_product_data(&new_product.name, &new_product.base_price)?;

        // Set default status if not provided
        if new_product.status.is_none() {
            new_product.status = Some(ProductStatus::InStock);
        }

        // Check if product with same name already exists
        let search_results = self.repository
            .search_by_name(&new_product.name, 0, 1)
            .await?;
        
        if !search_results.is_empty() {
            return Err(Error::with_message(
                ErrorCode::ResourceAlreadyExists,
                "Product with this name already exists",
            ));
        }

        self.repository.create(new_product).await
    }

    pub async fn get_product(&self, product_id: i64) -> Result<Product, Error> {
        if product_id <= 0 {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Invalid product ID",
            ));
        }

        self.repository.find_by_id(product_id).await
    }

    pub async fn list_products(&self, page: u32, page_size: u32) -> Result<ProductListResponse, Error> {
        if page_size == 0 || page_size > 100 {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Page size must be between 1 and 100",
            ));
        }

        let offset = (page.saturating_sub(1) * page_size) as i64;
        let limit = page_size as i64;

        let products = self.repository.find_all(offset, limit).await?;
        let total = self.repository.count_total().await?;

        Ok(ProductListResponse {
            products,
            total,
            page,
            page_size,
            total_pages: ((total as f64) / (page_size as f64)).ceil() as u32,
        })
    }

    pub async fn update_product(&self, product_id: i64, update_product: UpdateProduct) -> Result<Product, Error> {
        if product_id <= 0 {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Invalid product ID",
            ));
        }

        // Validate updated data if provided
        if let Some(ref name) = update_product.name {
            if let Some(ref base_price) = update_product.base_price {
                self.validate_product_data(name, base_price)?;
            }
        }

        // Check if product exists
        self.repository.find_by_id(product_id).await?;

        self.repository.update(product_id, update_product).await
    }

    pub async fn delete_product(&self, product_id: i64) -> Result<(), Error> {
        if product_id <= 0 {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Invalid product ID",
            ));
        }

        // Check if product exists
        self.repository.find_by_id(product_id).await?;

        self.repository.delete(product_id).await
    }

    pub async fn search_products(&self, name: &str, page: u32, page_size: u32) -> Result<Vec<ProductListItem>, Error> {
        if name.trim().is_empty() {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Search name cannot be empty",
            ));
        }

        if page_size == 0 || page_size > 100 {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Page size must be between 1 and 100",
            ));
        }

        let offset = (page.saturating_sub(1) * page_size) as i64;
        let limit = page_size as i64;

        self.repository.search_by_name(name, offset, limit).await
    }

    fn validate_product_data(&self, name: &str, base_price: &BigDecimal) -> Result<(), Error> {
        if name.trim().is_empty() {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Product name is required",
            ));
        }

        if name.len() > 150 {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Product name must be 150 characters or less",
            ));
        }

        if *base_price <= BigDecimal::from(0) {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Product price must be greater than 0",
            ));
        }

        Ok(())
    }
}

#[derive(Debug, serde::Serialize)]
pub struct ProductListResponse {
    pub products: Vec<ProductListItem>,
    pub total: i64,
    pub page: u32,
    pub page_size: u32,
    pub total_pages: u32,
}
