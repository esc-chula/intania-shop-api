use super::entity::{NewUser, User};
use crate::core::user::diesel::{NewUserModel, UserModel};
use crate::utils::db::{Pool, get_connection};
use crate::utils::errors::{Error, ErrorCode};
use async_trait::async_trait;
use diesel::prelude::*;

#[async_trait]
pub trait Repository {
    async fn create(&self, new_user: NewUser) -> Result<User, Error>;
    async fn find_by_email(&self, email: &str) -> Result<Option<User>, Error>;
}

pub struct DieselRepo {
    pool: Pool,
}

impl DieselRepo {
    pub fn new(pool: Pool) -> Self {
        DieselRepo { pool }
    }
}

#[async_trait]
impl Repository for DieselRepo {
    async fn create(&self, new_user: NewUser) -> Result<User, Error> {
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        let new_user_model = NewUserModel::from(new_user);
        let user_model: UserModel = diesel::insert_into(crate::schema::users::table)
            .values(&new_user_model)
            .get_result(&mut conn)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        Ok(User::from(user_model))
    }

    async fn find_by_email(&self, user_email: &str) -> Result<Option<User>, Error> {
        use crate::schema::users::dsl::*;
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        match users
            .filter(email.eq(user_email))
            .first::<UserModel>(&mut conn)
        {
            Ok(user_model) => Ok(Some(User::from(user_model))),
            Err(diesel::result::Error::NotFound) => Ok(None),
            Err(e) => Err(Error::with_message(ErrorCode::DatabaseError, e.to_string())),
        }
    }
}
