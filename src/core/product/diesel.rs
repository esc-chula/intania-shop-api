use async_trait::async_trait;
use bigdecimal::BigDecimal;
use diesel::prelude::*;
use serde::{Deserialize, Serialize};

use super::entity::{NewProduct, Product, ProductListItem, ProductStatus, UpdateProduct};
use super::repository::ProductRepository;
use crate::schema::products;
use crate::utils::db::DBPool;
use crate::utils::errors::{Error, ErrorCode};

#[derive(Debug, Clone, Queryable, Selectable, Serialize, Deserialize)]
#[diesel(table_name = products)]
#[diesel(check_for_backend(diesel::pg::Pg))]
pub struct ProductModel {
    pub product_id: i64,
    pub name: String,
    pub description: Option<String>,
    pub base_price: BigDecimal,
    pub status: ProductStatus,
}

#[derive(Debug, Clone, Insertable, Serialize, Deserialize)]
#[diesel(table_name = products)]
pub struct NewProductModel {
    pub name: String,
    pub description: Option<String>,
    pub base_price: BigDecimal,
    pub status: ProductStatus,
}

#[derive(Debug, Clone, AsChangeset, Serialize, Deserialize)]
#[diesel(table_name = products)]
pub struct UpdateProductModel {
    pub name: Option<String>,
    pub description: Option<String>,
    pub base_price: Option<BigDecimal>,
    pub status: Option<ProductStatus>,
}

impl From<ProductModel> for Product {
    fn from(model: ProductModel) -> Self {
        Product {
            product_id: model.product_id,
            name: model.name,
            description: model.description,
            base_price: model.base_price,
            status: model.status,
        }
    }
}

impl From<ProductModel> for ProductListItem {
    fn from(model: ProductModel) -> Self {
        ProductListItem {
            product_id: model.product_id,
            name: model.name,
            base_price: model.base_price,
            status: model.status,
        }
    }
}

impl From<NewProduct> for NewProductModel {
    fn from(new_product: NewProduct) -> Self {
        NewProductModel {
            name: new_product.name,
            description: new_product.description,
            base_price: new_product.base_price,
            status: new_product.status.unwrap_or_default(),
        }
    }
}

impl From<UpdateProduct> for UpdateProductModel {
    fn from(update_product: UpdateProduct) -> Self {
        UpdateProductModel {
            name: update_product.name,
            description: update_product.description,
            base_price: update_product.base_price,
            status: update_product.status,
        }
    }
}

pub struct DieselProductRepository {
    pool: DBPool,
}

impl DieselProductRepository {
    pub fn new(pool: DBPool) -> Self {
        Self { pool }
    }
}

#[async_trait]
impl ProductRepository for DieselProductRepository {
    async fn create(&self, new_product: NewProduct) -> Result<Product, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        let new_product_model: NewProductModel = new_product.into();

        let product_model: ProductModel = diesel::insert_into(products::table)
            .values(&new_product_model)
            .returning(ProductModel::as_returning())
            .get_result(&mut conn)
            .map_err(|e| {
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to create product: {}", e),
                )
            })?;

        Ok(product_model.into())
    }

    async fn find_by_id(&self, product_id: i64) -> Result<Product, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        let product_model: ProductModel = products::table
            .filter(products::product_id.eq(product_id))
            .select(ProductModel::as_select())
            .first(&mut conn)
            .map_err(|e| match e {
                diesel::result::Error::NotFound => {
                    Error::with_message(ErrorCode::ResourceNotFound, "Product not found")
                }
                _ => {
                    Error::with_message(ErrorCode::DatabaseError, format!("Database error: {}", e))
                }
            })?;

        Ok(product_model.into())
    }

    async fn find_all(&self, offset: i64, limit: i64) -> Result<Vec<ProductListItem>, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        let product_models: Vec<ProductModel> = products::table
            .select(ProductModel::as_select())
            .order(products::product_id.desc())
            .offset(offset)
            .limit(limit)
            .load(&mut conn)
            .map_err(|e| {
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to fetch products: {}", e),
                )
            })?;

        Ok(product_models
            .into_iter()
            .map(|model| model.into())
            .collect())
    }

    async fn update(
        &self,
        product_id: i64,
        update_product: UpdateProduct,
    ) -> Result<Product, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        let update_model: UpdateProductModel = update_product.into();

        let product_model: ProductModel = diesel::update(products::table)
            .filter(products::product_id.eq(product_id))
            .set(&update_model)
            .returning(ProductModel::as_returning())
            .get_result(&mut conn)
            .map_err(|e| match e {
                diesel::result::Error::NotFound => {
                    Error::with_message(ErrorCode::ResourceNotFound, "Product not found")
                }
                _ => Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to update product: {}", e),
                ),
            })?;

        Ok(product_model.into())
    }

    async fn delete(&self, product_id: i64) -> Result<(), Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        let deleted_rows = diesel::delete(products::table)
            .filter(products::product_id.eq(product_id))
            .execute(&mut conn)
            .map_err(|e| {
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to delete product: {}", e),
                )
            })?;

        if deleted_rows == 0 {
            return Err(Error::with_message(
                ErrorCode::ResourceNotFound,
                "Product not found",
            ));
        }

        Ok(())
    }

    async fn search_by_name(
        &self,
        name: &str,
        offset: i64,
        limit: i64,
    ) -> Result<Vec<ProductListItem>, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        let search_pattern = format!("%{}%", name);

        let product_models: Vec<ProductModel> = products::table
            .filter(products::name.ilike(&search_pattern))
            .select(ProductModel::as_select())
            .order(products::product_id.desc())
            .offset(offset)
            .limit(limit)
            .load(&mut conn)
            .map_err(|e| {
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to search products: {}", e),
                )
            })?;

        Ok(product_models
            .into_iter()
            .map(|model| model.into())
            .collect())
    }

    async fn count_total(&self) -> Result<i64, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        let count = products::table
            .count()
            .get_result::<i64>(&mut conn)
            .map_err(|e| {
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to count products: {}", e),
                )
            })?;

        Ok(count)
    }
}
