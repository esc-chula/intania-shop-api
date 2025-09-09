use chrono::prelude::*;
use jsonwebtoken::{Algorithm, DecodingKey, EncodingKey, Header, Validation};
use axum::{
    extract::Request,
    http::{header, StatusCode},
    middleware::Next,
    response::Response,
};
use serde::{Deserialize, Serialize};
use std::env;

use crate::utils::errors::{Error, ErrorCode};
use crate::core::user::entity::Role;

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
            _ => iat + chrono::Duration::days(1),
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

    pub fn from(token: String) -> Result<Claims, Error> {
        let key = env::var("JWT_PASSWORD").unwrap();
        match jsonwebtoken::decode::<Claims>(
            &token,
            &DecodingKey::from_secret(key.as_bytes()),
            &Validation::new(Algorithm::HS512),
        ) {
            Ok(token_data) => Ok(token_data.claims),
            Err(_) => Err(Error::new(ErrorCode::NotAuthorized)),
        }
    }
}

/// Axum middleware for JWT authentication
pub async fn auth_middleware(
    mut request: Request,
    next: Next,
) -> Result<Response, StatusCode> {
    let auth_header = request
        .headers()
        .get(header::AUTHORIZATION)
        .and_then(|value| value.to_str().ok());

    let token = match auth_header {
        Some(auth_str) if auth_str.starts_with("Bearer ") => {
            auth_str.trim_start_matches("Bearer ").to_string()
        }
        _ => return Err(StatusCode::UNAUTHORIZED),
    };

    match Claims::from(token) {
        Ok(claims) => {
            request.extensions_mut().insert(claims);
            Ok(next.run(request).await)
        }
        Err(_) => Err(StatusCode::UNAUTHORIZED),
    }
}

#[derive(Debug)]
pub struct AuthenticatedUser(pub Claims);

impl AuthenticatedUser {
    pub fn claims(&self) -> &Claims {
        &self.0
    }
}
