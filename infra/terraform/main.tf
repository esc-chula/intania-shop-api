locals {
  api_name             = "intania-shop-api"
  artifact_repo_id     = local.api_name
  cloud_sql_name       = "${local.api_name}-db"
  cloud_run_job        = "${local.api_name}-migrate"
  upload_bucket        = "${var.project_id}-uploads"
  runtime_sa_name      = "intania-api"
  cloud_build_ci_sa_id = "intania-cloudbuild-ci"
  cloud_build_sa_id    = "intania-cloudbuild"

  required_services = toset([
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "iamcredentials.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "sqladmin.googleapis.com",
  ])

  application_secrets = toset([
    "database-url",
    "google-client-id",
    "google-client-secret",
    "jwt-secret",
  ])
}

resource "google_project_service" "required" {
  for_each = local.required_services

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_artifact_registry_repository" "api" {
  project       = var.project_id
  location      = var.region
  repository_id = local.artifact_repo_id
  description   = "Container images for ${local.api_name}"
  format        = "DOCKER"

  depends_on = [google_project_service.required]
}

resource "google_storage_bucket" "uploads" {
  project                     = var.project_id
  name                        = local.upload_bucket
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  versioning {
    enabled = true
  }

  depends_on = [google_project_service.required]
}

resource "google_service_account" "runtime" {
  project      = var.project_id
  account_id   = local.runtime_sa_name
  display_name = "Intania Shop API runtime"
}

resource "google_service_account" "cloud_build" {
  project      = var.project_id
  account_id   = local.cloud_build_sa_id
  display_name = "Intania Shop API Cloud Build deployer"
}

# CI receives no deployment capability. A pull request can change its own
# cloudbuild/ci.yaml, so this identity must remain unable to reach production.
resource "google_service_account" "cloud_build_ci" {
  project      = var.project_id
  account_id   = local.cloud_build_ci_sa_id
  display_name = "Intania Shop API Cloud Build CI"
}

resource "google_secret_manager_secret" "application" {
  for_each = local.application_secrets

  project   = var.project_id
  secret_id = "${local.api_name}-${each.value}"

  replication {
    auto {}
  }

  depends_on = [google_project_service.required]
}

# Production database credentials are intentionally not Terraform variables or
# resources. A secret value here would be stored in Terraform state.
resource "google_sql_database_instance" "primary" {
  project             = var.project_id
  name                = local.cloud_sql_name
  region              = var.region
  database_version    = "POSTGRES_16"
  deletion_protection = true

  settings {
    tier    = var.sql_tier
    edition = "ENTERPRISE"

    availability_type = "ZONAL"
    disk_type         = "PD_SSD"
    disk_size         = 10
    disk_autoresize   = true

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
      transaction_log_retention_days = 7
    }

    # Cloud Run attaches the Cloud SQL connector over this public endpoint.
    # No authorised networks are configured, so direct client IP access is off.
    ip_configuration {
      ipv4_enabled = true
    }

    deletion_protection_enabled = true
  }

  depends_on = [google_project_service.required]
}

resource "google_sql_database" "application" {
  project  = var.project_id
  name     = "intania_shop"
  instance = google_sql_database_instance.primary.name
}

resource "google_project_iam_member" "runtime_cloud_sql" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_storage_bucket_iam_member" "runtime_uploads" {
  bucket = google_storage_bucket.uploads.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_secret_manager_secret_iam_member" "runtime_secret_access" {
  for_each = google_secret_manager_secret.application

  project   = var.project_id
  secret_id = each.value.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_project_iam_member" "build_roles" {
  for_each = toset([
    "roles/artifactregistry.writer",
    "roles/cloudbuild.builds.builder",
    "roles/logging.logWriter",
    "roles/run.admin",
  ])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.cloud_build.email}"
}

resource "google_project_iam_member" "build_ci_roles" {
  for_each = toset([
    "roles/cloudbuild.builds.builder",
    "roles/logging.logWriter",
  ])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.cloud_build_ci.email}"
}

resource "google_service_account_iam_member" "build_can_run_as_runtime" {
  service_account_id = google_service_account.runtime.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.cloud_build.email}"
}
