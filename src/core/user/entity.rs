use chrono::NaiveDateTime;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug, Clone, PartialEq)]
pub enum Role {
    User,
    Admin,
}

impl From<String> for Role {
    fn from(s: String) -> Self {
        match s.as_str() {
            "ADMIN" => Role::Admin,
            _ => Role::User,
        }
    }
}

impl From<Role> for String {
    fn from(role: Role) -> Self {
        match role {
            Role::Admin => "ADMIN".to_string(),
            Role::User => "USER".to_string(),
        }
    }
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct User {
    pub id: i64,
    pub full_name: String,
    pub email: String,
    pub password_hash: String,
    pub phone: Option<String>,
    pub role: Role,
    pub created_at: NaiveDateTime,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct NewUser {
    pub full_name: String,
    pub email: String,
    pub password_hash: String,
    pub phone: Option<String>,
    pub role: Role,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct AbstractUser {
    pub id: i64,
    pub full_name: String,
    pub email: String,
    pub role: Role,
}

impl From<User> for AbstractUser {
    fn from(user: User) -> Self {
        AbstractUser {
            id: user.id,
            full_name: user.full_name,
            email: user.email,
            role: user.role,
        }
    }
}

