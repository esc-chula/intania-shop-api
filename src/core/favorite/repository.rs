use async_trait::async_trait;

use crate::utils::errors::Error;

use super::entity::{AddFavoriteRequest, Favorite};

#[async_trait]
pub trait FavoriteRepository: Send + Sync {
    async fn add(&self, req: AddFavoriteRequest) -> Result<Favorite, Error>;
}
