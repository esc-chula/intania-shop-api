use crate::utils::errors::{Error, ErrorCode};
use crate::utils::db::{get_connection, Pool};
use super::entity::{User, NewUser};
use diesel::prelude::*;
use crate::core::user::diesel::{UserModel, NewUserModel};
use async_trait::async_trait;

#[async_trait]
pub trait Repository {
    async fn add(&self, user: User) -> Result<(), Error>;
    async fn create(&self, new_user: NewUser) -> Result<User, Error>;
    async fn find_by_email(&self, email: &str) -> Result<Option<User>, Error>;
    async fn find(&self, user_id: String) -> Result<User, Error>;
    async fn remove(&self, user_id: String) -> Result<(), Error>;
    async fn update(&self, user: User) -> Result<(), Error>;
    async fn get_paged(&self, skip: i64, limit: i64) -> Result<Vec<User>, Error>;
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
    async fn add(&self, user: User) -> Result<(), Error> {
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        diesel::insert_into(crate::schema::users::table)
            .values(&NewUserModel::from(user))
            .execute(&mut conn)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        Ok(())
    }

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

    async fn find(&self, user_id_str: String) -> Result<User, Error> {
        use crate::schema::users::dsl::*;
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        let id: i64 = user_id_str
            .parse()
            .map_err(|_| Error::new(ErrorCode::BadRequest))?;

        users
            .find(id)
            .first::<UserModel>(&mut conn)
            .map_err(|e| match e {
                diesel::result::Error::NotFound => Error::new(ErrorCode::ResourceNotFound),
                _ => Error::with_message(ErrorCode::DatabaseError, e.to_string()),
            })
            .map(User::from)
    }

    async fn remove(&self, user_id_str: String) -> Result<(), Error> {
        use crate::schema::users::dsl::*;
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        let id: i64 = user_id_str
            .parse()
            .map_err(|_| Error::new(ErrorCode::BadRequest))?;

        diesel::delete(users.find(id))
            .execute(&mut conn)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        Ok(())
    }

    async fn update(&self, user: User) -> Result<(), Error> {
        use crate::schema::users::dsl::*;
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        diesel::update(users.find(user.id))
            .set(NewUserModel::from(user))
            .execute(&mut conn)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        Ok(())
    }

    async fn get_paged(&self, skip: i64, limit: i64) -> Result<Vec<User>, Error> {
        use crate::schema::users::dsl::*;
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        users
            .order(user_id.asc())
            .offset(skip)
            .limit(limit)
            .load::<UserModel>(&mut conn)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))
            .map(|rows| rows.into_iter().map(User::from).collect())
    }
}
