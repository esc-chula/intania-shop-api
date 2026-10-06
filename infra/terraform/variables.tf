variable "project_id" {
  description = "The dedicated production GCP project ID."
  type        = string
}

variable "region" {
  description = "Primary GCP region for Cloud Run, Artifact Registry, and uploads."
  type        = string
  default     = "asia-southeast1"
}

variable "github_owner" {
  description = "GitHub organization or user that owns the API repository."
  type        = string
  default     = "esc-chula"
}

variable "github_repo" {
  description = "GitHub repository name, without the owner."
  type        = string
  default     = "intania-shop-api"
}

variable "sql_tier" {
  description = "Paid Cloud SQL machine tier. Confirm this cost before apply."
  type        = string
}

variable "cors_allowed_origins" {
  description = "Comma-separated HTTPS frontend origins allowed by the API."
  type        = string

  validation {
    condition     = length(trimspace(var.cors_allowed_origins)) > 0
    error_message = "cors_allowed_origins must contain the real frontend origin before creating the release trigger."
  }
}

variable "frontend_callback_url" {
  description = "Exact frontend URL that receives the short-lived OAuth login code after Google sign-in."
  type        = string

  validation {
    condition     = can(regex("^https://", var.frontend_callback_url))
    error_message = "frontend_callback_url must be an HTTPS frontend callback URL."
  }
}

variable "google_redirect_url" {
  description = "Exact Google OAuth callback URL exposed by Cloud Run."
  type        = string

  validation {
    condition     = can(regex("^https://", var.google_redirect_url))
    error_message = "google_redirect_url must be an HTTPS URL registered in Google OAuth."
  }
}

variable "jwt_issuer" {
  description = "Issuer placed in application JWTs."
  type        = string
  default     = "intania-shop-api"
}
