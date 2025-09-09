use crate::utils::errors::{Error, ErrorCode};
use crate::utils::db::{get_connection, Pool};
use super::entity::User;
use diesel::prelude::*;
use crate::core::user::diesel::{UserModel, NewUserModel};

pub trait Repository {
    fn add(&self, user: User) -> Result<(), Error>;
    fn find(&self, user_id: String) -> Result<User, Error>;
    fn remove(&self, user_id: String) -> Result<(), Error>;
    fn update(&self, user: User) -> Result<(), Error>;
    fn get_paged(&self, skip: i64, limit: i64) -> Result<Vec<User>, Error>;
}

pub struct DieselRepo {
    pool: Pool,
}

impl DieselRepo {
    pub fn new(pool: Pool) -> Self {
        DieselRepo { pool }
    }
}

impl Repository for DieselRepo {
    fn add(&self, user: User) -> Result<(), Error> {
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        diesel::insert_into(crate::schema::users::table)
            .values(&NewUserModel::from(user))
            .execute(&mut conn)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        Ok(())
    }

    fn find(&self, user_id_str: String) -> Result<User, Error> {
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

    fn remove(&self, user_id_str: String) -> Result<(), Error> {
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

    fn update(&self, user: User) -> Result<(), Error> {
        use crate::schema::users::dsl::*;
        let mut conn = get_connection(&self.pool)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        diesel::update(users.find(user.id))
            .set(NewUserModel::from(user))
            .execute(&mut conn)
            .map_err(|e| Error::with_message(ErrorCode::DatabaseError, e.to_string()))?;

        Ok(())
    }

    fn get_paged(&self, skip: i64, limit: i64) -> Result<Vec<User>, Error> {
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
