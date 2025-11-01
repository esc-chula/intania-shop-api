use chrono::prelude::*;
use jsonwebtoken::{Algorithm, EncodingKey, Header};
use serde::{Deserialize, Serialize};
use std::env;

use crate::core::user::entity::Role;
use crate::utils::errors::{Error, ErrorCode};

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct Claims {
    pub id: String,
    pub role: Role,
    #[serde(with = "date_serializer")]
    iat: DateTime<Utc>,
    #[serde(with = "date_serializer")]
    exp: DateTime<Utc>,
}

mod date_serializer {
    use chrono::{DateTime, TimeZone, Utc};
    use serde::{self, Deserialize, Deserializer, Serializer};

    pub fn serialize<S>(date: &DateTime<Utc>, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        let timestamp = date.timestamp();
        serializer.serialize_i64(timestamp)
    }

    pub fn deserialize<'de, D>(deserializer: D) -> Result<DateTime<Utc>, D::Error>
    where
        D: Deserializer<'de>,
    {
        Utc.timestamp_opt(i64::deserialize(deserializer)?, 0)
            .single()
            .ok_or_else(|| serde::de::Error::custom("invalid Unix timestamp value"))
    }
}

impl Claims {
    pub fn new(username: String, role: Role) -> Claims {
        let iat = Utc::now();
        let exp = match role {
            Role::Admin => iat + chrono::Duration::minutes(30),
            Role::User => iat + chrono::Duration::days(1),
        };

        Claims {
            id: username,
            role,
            iat,
            exp,
        }
    }

    pub fn jwt(&self) -> Result<String, Error> {
        let mut header = Header::default();
        header.alg = Algorithm::HS512;
        header.kid = Some(env::var("JWT_SIGNING_KEY").unwrap());
        let key = env::var("JWT_PASSWORD").unwrap();

        match jsonwebtoken::encode(&header, self, &EncodingKey::from_secret(key.as_bytes())) {
            Ok(token) => Ok(token),
            Err(_) => Err(Error::new(ErrorCode::InternalError)),
        }
    }
}
