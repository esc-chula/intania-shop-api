use std::sync::Arc;

use crate::utils::errors::Error;

use super::entity::{AddFavoriteRequest, AddFavoriteResponse};
use super::repository::FavoriteRepository;

pub struct FavoriteService {
    repo: Arc<dyn FavoriteRepository>,
}

impl FavoriteService {
    pub fn new(repo: Arc<dyn FavoriteRepository>) -> Self {
        Self { repo }
    }

    pub async fn add(&self, req: AddFavoriteRequest) -> Result<AddFavoriteResponse, Error> {
        let fav = self.repo.add(req).await?;
        Ok(AddFavoriteResponse {
            user_id: fav.user_id,
            product_id: fav.product_id,
            created_at: fav.created_at,
            message: "Added to favorites".into(),
        })
    }
}
