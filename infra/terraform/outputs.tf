output "artifact_registry_repository" {
  value       = google_artifact_registry_repository.api.name
  description = "Artifact Registry repository path."
}

output "cloud_sql_connection_name" {
  value       = google_sql_database_instance.primary.connection_name
  description = "Cloud SQL connection name for Cloud Run attachment and DATABASE_URL."
}

output "runtime_service_account" {
  value       = google_service_account.runtime.email
  description = "Service account attached to Cloud Run and the migration job."
}

output "upload_bucket" {
  value       = google_storage_bucket.uploads.name
  description = "Private application upload bucket."
}

output "secret_ids" {
  value       = { for name, secret in google_secret_manager_secret.application : name => secret.secret_id }
  description = "Secret containers that need a value added outside Terraform."
}
