use diesel::prelude::*;
use diesel::pg::Pg;
use diesel::serialize::{self, ToSql};
use diesel::deserialize::{self, FromSql};
use diesel::sql_types::Text;
use std::io::Write;

use crate::schema::users;

use super::entity::{User, NewUser};


use diesel_derive_enum::DbEnum;

#[derive(DbEnum, Debug, Clone, PartialEq, Eq)]
#[ExistingTypePath = "crate::schema::sql_types::UserRole"]
pub enum DbRole {
    #[db_rename = "USER"]
    User,
    #[db_rename = "ADMIN"]
    Admin,
}

use super::entity::Role as DomainRole;

impl From<DbRole> for DomainRole {
    fn from(db_role: DbRole) -> Self {
        match db_role {
            DbRole::User => DomainRole::User,
            DbRole::Admin => DomainRole::Admin,
        }
    }
}

impl From<DomainRole> for DbRole {
    fn from(role: DomainRole) -> Self {
        match role {
            DomainRole::User => DbRole::User,
            DomainRole::Admin => DbRole::Admin,
        }
    }
}

#[derive(Queryable, Identifiable, Debug)]
#[diesel(table_name = users)]
#[diesel(primary_key(user_id))]
pub struct UserModel {
    pub user_id: i64,
    pub full_name: Option<String>,
    pub email: String,
    pub password_hash: String,
    pub phone: Option<String>,
    pub role: DbRole,  // DB enum
    pub created_at: chrono::NaiveDateTime,
    pub google_sub: Option<String>,
    pub google_picture: Option<String>,
    pub email_verified: bool,
}

impl From<UserModel> for super::entity::User {
    fn from(model: UserModel) -> Self {
        super::entity::User {
            id: model.user_id,
            full_name: model.full_name.unwrap_or_default(),
            email: model.email,
            password_hash: model.password_hash,
            phone: model.phone,
            role: model.role.into(), // DbRole -> DomainRole
            created_at: model.created_at,
        }
    }
}

#[derive(Insertable, Debug, AsChangeset)]
#[diesel(table_name = users)]
pub struct NewUserModel {
    pub full_name: Option<String>,
    pub email: String,
    pub password_hash: String,
    pub phone: Option<String>,
    pub role: DbRole, 
}

impl From<super::entity::User> for NewUserModel {
    fn from(user: super::entity::User) -> Self {
        NewUserModel {
            full_name: if user.full_name.is_empty() { None } else { Some(user.full_name) },
            email: user.email,
            password_hash: user.password_hash,
            phone: user.phone,
            role: user.role.into(), // DomainRole -> DbRole
        }
    }
}

impl From<super::entity::NewUser> for NewUserModel {
    fn from(new_user: super::entity::NewUser) -> Self {
        NewUserModel {
            full_name: if new_user.full_name.is_empty() { None } else { Some(new_user.full_name) },
            email: new_user.email,
            password_hash: new_user.password_hash,
            phone: new_user.phone,
            role: new_user.role.into(), // DomainRole -> DbRole
        }
    }
}
