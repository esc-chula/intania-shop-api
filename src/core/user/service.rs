use super::entity::{User, AbstractUser};
use super::repository::Repository;
use crate::utils::errors::{Error, ErrorCode};

pub struct Service {
    repo: Box<dyn Repository + Send + Sync>,
}

impl Service {
    pub fn new(repo: Box<dyn Repository + Send + Sync>) -> Self {
        Service { repo }
    }

    pub fn add(&self, user: User) -> Result<(), Error> {
        if user.email.trim().is_empty() {
            return Err(Error::with_message(ErrorCode::ValidationError, "Email is required"));
        }
        if user.password_hash.trim().is_empty() {
            return Err(Error::with_message(ErrorCode::ValidationError, "Password is required"));
        }

        if self.repo.find(user.id.to_string()).is_ok() {
            return Err(Error::new(ErrorCode::ResourceAlreadyExists));
        }

        self.repo.add(user)
    }

    pub fn remove(&self, user_id: String) -> Result<(), Error> {
        if user_id.trim().is_empty() {
            return Err(Error::with_message(ErrorCode::ValidationError, "User ID is required"));
        }

        self.repo.remove(user_id)
    }

    pub fn update(&self, user: User) -> Result<(), Error> {
        if user.email.trim().is_empty() {
            return Err(Error::with_message(ErrorCode::ValidationError, "Email is required"));
        }

        self.repo.update(user)
    }

    pub fn retrieve(&self, skip: i64, limit: i64) -> Result<Vec<AbstractUser>, Error> {
        if skip < 0 {
            return Err(Error::with_message(ErrorCode::ValidationError, "Skip must be non-negative"));
        }
        if limit <= 0 || limit > 100 {
            return Err(Error::with_message(ErrorCode::ValidationError, "Limit must be between 1 and 100"));
        }

        let users = self.repo.get_paged(skip, limit)?;
        Ok(users.into_iter().map(AbstractUser::from).collect())
    }

    pub fn find(&self, user_id: String) -> Result<User, Error> {
        if user_id.trim().is_empty() {
            return Err(Error::with_message(ErrorCode::ValidationError, "User ID is required"));
        }

        self.repo.find(user_id)
    }
}
