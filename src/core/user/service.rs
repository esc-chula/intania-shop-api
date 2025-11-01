use super::entity::{
    AbstractUser, LoginRequest, LoginResponse, NewUser, RegistrationResponse, Role,
    UserRegistration,
};
use super::repository::Repository;
use crate::api::guards::guard::Claims;
use crate::utils::errors::{Error, ErrorCode};
use bcrypt::{DEFAULT_COST, hash, verify};
use std::sync::Arc;

#[derive(Clone)]
pub struct Service {
    repo: Arc<dyn Repository + Send + Sync>,
}

impl Service {
    pub fn new(repo: Arc<dyn Repository + Send + Sync>) -> Self {
        Service { repo }
    }

    pub async fn register(
        &self,
        registration: UserRegistration,
    ) -> Result<RegistrationResponse, Error> {
        // Validate input
        if registration.full_name.trim().is_empty() {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Full name is required",
            ));
        }
        if registration.email.trim().is_empty() {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Email is required",
            ));
        }
        if registration.password.len() < 6 {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Password must be at least 6 characters",
            ));
        }
        if registration.password != registration.confirm_password {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Passwords do not match",
            ));
        }

        // Check if user already exists
        if let Ok(Some(_)) = self.repo.find_by_email(&registration.email).await {
            return Err(Error::with_message(
                ErrorCode::ResourceAlreadyExists,
                "User with this email already exists",
            ));
        }

        // Hash password
        let password_hash = hash(&registration.password, DEFAULT_COST).map_err(|_| {
            Error::with_message(ErrorCode::InternalError, "Failed to hash password")
        })?;

        // Create new user
        let new_user = NewUser {
            full_name: registration.full_name,
            email: registration.email,
            password_hash,
            phone: registration.phone,
            role: Role::User, // Default role
        };

        // Save to database
        let user = self.repo.create(new_user).await?;

        Ok(RegistrationResponse {
            user: AbstractUser::from(user),
            message: "User registered successfully".to_string(),
        })
    }

    pub async fn login(&self, login_request: LoginRequest) -> Result<LoginResponse, Error> {
        // Validate input
        if login_request.email.trim().is_empty() {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Email is required",
            ));
        }
        if login_request.password.trim().is_empty() {
            return Err(Error::with_message(
                ErrorCode::ValidationError,
                "Password is required",
            ));
        }

        // Find user by email
        let user = match self.repo.find_by_email(&login_request.email).await? {
            Some(user) => user,
            None => {
                return Err(Error::with_message(
                    ErrorCode::InvalidCredentials,
                    "Invalid email or password",
                ));
            }
        };

        // Verify password
        if !verify(&login_request.password, &user.password_hash).map_err(|_| {
            Error::with_message(ErrorCode::InternalError, "Failed to verify password")
        })? {
            return Err(Error::with_message(
                ErrorCode::InvalidCredentials,
                "Invalid email or password",
            ));
        }

        let claims = Claims::new(user.email.clone(), user.role.clone());
        let token = claims.jwt().map_err(|_| {
            Error::with_message(ErrorCode::InternalError, "Failed to generate JWT token")
        })?;

        Ok(LoginResponse {
            user: AbstractUser::from(user),
            token,
            message: "Login successful".to_string(),
        })
    }
}
