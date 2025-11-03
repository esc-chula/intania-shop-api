use async_trait::async_trait;
use bigdecimal::BigDecimal;
use diesel::prelude::*;
use serde::{Deserialize, Serialize};

use super::entity::{
    NewProduct, Product, ProductDetail, ProductListItem, ProductStatus, UpdateProduct, Variant,
};
use super::repository::ProductRepository;
use crate::schema::{products, variants};
use crate::utils::db::DBPool;
use crate::utils::errors::{Error, ErrorCode};

#[derive(Debug, Clone, Queryable, Selectable, Serialize, Deserialize)]
#[diesel(table_name = products)]
#[diesel(check_for_backend(diesel::pg::Pg))]
pub struct ProductModel {
    pub id: i64,
    pub name: String,
    pub description: Option<String>,
    pub price: BigDecimal,
    pub status: ProductStatus,
    pub category: Option<String>,
    pub stock_quantity: Option<i32>,
    pub created_at: Option<chrono::NaiveDateTime>,
    pub updated_at: Option<chrono::NaiveDateTime>,
    pub preview_image: Option<Vec<Option<String>>>,
    pub preview_video: Option<Vec<Option<String>>>,
    pub shipping: Option<Vec<Option<String>>>,
}

#[derive(Debug, Clone, Insertable, Serialize, Deserialize)]
#[diesel(table_name = products)]
pub struct NewProductModel {
    pub name: String,
    pub description: Option<String>,
    pub price: BigDecimal,
    pub status: ProductStatus,
    pub category: Option<String>,
    pub stock_quantity: Option<i32>,
    pub preview_image: Option<Vec<Option<String>>>,
    pub preview_video: Option<Vec<Option<String>>>,
    pub shipping: Option<Vec<Option<String>>>,
}

#[derive(Debug, Clone, AsChangeset, Serialize, Deserialize)]
#[diesel(table_name = products)]
pub struct UpdateProductModel {
    pub name: Option<String>,
    pub description: Option<String>,
    pub price: Option<BigDecimal>,
    pub status: Option<ProductStatus>,
    pub category: Option<String>,
    pub stock_quantity: Option<i32>,
    pub preview_image: Option<Vec<Option<String>>>,
    pub preview_video: Option<Vec<Option<String>>>,
    pub shipping: Option<Vec<Option<String>>>,
}

#[derive(Debug, Clone, Queryable, Selectable, Serialize, Deserialize)]
#[diesel(table_name = variants)]
#[diesel(check_for_backend(diesel::pg::Pg))]
pub struct VariantModel {
    pub variant_id: i64,
    pub product_id: i64,
    pub size: Option<String>,
    pub color: Option<String>,
    pub stock_quantity: Option<i32>,
}

impl From<ProductModel> for Product {
    fn from(model: ProductModel) -> Self {
        Product {
            product_id: model.id,
            name: model.name,
            description: model.description,
            base_price: model.price,
            status: model.status,
            category: model.category,
            stock_quantity: model.stock_quantity,
            preview_image: model.preview_image,
            preview_video: model.preview_video,
            shipping: model.shipping,
        }
    }
}

impl From<ProductModel> for ProductListItem {
    fn from(model: ProductModel) -> Self {
        ProductListItem {
            product_id: model.id,
            name: model.name,
            base_price: model.price,
            status: model.status,
            category: model.category,
            preview_image: model.preview_image,
        }
    }
}

impl From<NewProduct> for NewProductModel {
    fn from(new_product: NewProduct) -> Self {
        NewProductModel {
            name: new_product.name,
            description: new_product.description,
            price: new_product.base_price,
            status: new_product.status.unwrap_or_default(),
            category: new_product.category,
            stock_quantity: new_product.stock_quantity,
            preview_image: new_product.preview_image,
            preview_video: new_product.preview_video,
            shipping: new_product.shipping,
        }
    }
}

impl From<UpdateProduct> for UpdateProductModel {
    fn from(update_product: UpdateProduct) -> Self {
        UpdateProductModel {
            name: update_product.name,
            description: update_product.description,
            price: update_product.base_price,
            status: update_product.status,
            category: update_product.category,
            stock_quantity: update_product.stock_quantity,
            preview_image: update_product.preview_image,
            preview_video: update_product.preview_video,
            shipping: update_product.shipping,
        }
    }
}

impl From<VariantModel> for Variant {
    fn from(model: VariantModel) -> Self {
        Variant {
            variant_id: model.variant_id,
            product_id: model.product_id,
            size: model.size,
            color: model.color,
            stock_quantity: model.stock_quantity,
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
            .filter(products::id.eq(product_id))
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
            .order(products::id.desc())
            .offset(offset)
            .limit(limit)
            .load(&mut conn)
            .map_err(|e| {
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to fetch products: {}", e),
                )
            })?;

        Ok(product_models.into_iter().map(Into::into).collect())
    }

    async fn find_by_id_with_variants(&self, product_id: i64) -> Result<ProductDetail, Error> {
        let mut conn = self.pool.get().map_err(|e| {
            Error::with_message(ErrorCode::DatabaseError, format!("Connection error: {}", e))
        })?;

        // Get product
        let product_model: ProductModel = products::table
            .filter(products::id.eq(product_id))
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

        // Get variants for this product
        let variant_models: Vec<VariantModel> = variants::table
            .filter(variants::product_id.eq(product_id))
            .select(VariantModel::as_select())
            .load(&mut conn)
            .map_err(|e| {
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to fetch variants: {}", e),
                )
            })?;

        let variants: Vec<Variant> = variant_models.into_iter().map(Into::into).collect();

        Ok(ProductDetail {
            product_id: product_model.id,
            name: product_model.name,
            description: product_model.description,
            base_price: product_model.price,
            status: product_model.status,
            category: product_model.category,
            stock_quantity: product_model.stock_quantity,
            preview_image: product_model.preview_image,
            preview_video: product_model.preview_video,
            shipping: product_model.shipping,
            variants,
        })
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
            .filter(products::id.eq(product_id))
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
            .filter(products::id.eq(product_id))
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
            .order(products::id.desc())
            .offset(offset)
            .limit(limit)
            .load(&mut conn)
            .map_err(|e| {
                Error::with_message(
                    ErrorCode::DatabaseError,
                    format!("Failed to search products: {}", e),
                )
            })?;

        Ok(product_models.into_iter().map(Into::into).collect())
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
