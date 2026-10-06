locals {
  build_substitutions = {
    _AR_REPOSITORY           = google_artifact_registry_repository.api.repository_id
    _CLOUD_SQL_INSTANCE      = google_sql_database_instance.primary.connection_name
    _CORS_ALLOWED_ORIGINS    = var.cors_allowed_origins
    _FRONTEND_CALLBACK_URL   = var.frontend_callback_url
    _GOOGLE_REDIRECT_URL     = var.google_redirect_url
    _JWT_ISSUER              = var.jwt_issuer
    _MIGRATION_JOB           = local.cloud_run_job
    _REGION                  = var.region
    _RUNTIME_SERVICE_ACCOUNT = google_service_account.runtime.email
    _SERVICE_NAME            = local.api_name
    _UPLOAD_BUCKET           = google_storage_bucket.uploads.name
  }
}

# Pull requests and main only validate. A tagged release is the only path to
# production, and every release must be explicitly approved in Cloud Build.
resource "google_cloudbuild_trigger" "pull_request_ci" {
  project         = var.project_id
  name            = "${local.api_name}-pr-ci"
  description     = "Run API checks for pull requests targeting main."
  filename        = "cloudbuild/ci.yaml"
  service_account = google_service_account.cloud_build_ci.id

  github {
    owner = var.github_owner
    name  = var.github_repo

    pull_request {
      branch          = "^main$"
      comment_control = "COMMENTS_DISABLED"
    }
  }

  depends_on = [google_project_service.required, google_project_iam_member.build_ci_roles]
}

resource "google_cloudbuild_trigger" "main_ci" {
  project         = var.project_id
  name            = "${local.api_name}-main-ci"
  description     = "Run API checks for main; this trigger never deploys."
  filename        = "cloudbuild/ci.yaml"
  service_account = google_service_account.cloud_build_ci.id

  github {
    owner = var.github_owner
    name  = var.github_repo

    push {
      branch = "^main$"
    }
  }

  depends_on = [google_project_service.required, google_project_iam_member.build_ci_roles]
}

resource "google_cloudbuild_trigger" "release" {
  project         = var.project_id
  name            = "${local.api_name}-release"
  description     = "Deploy a version tag to production after Cloud Build approval."
  filename        = "cloudbuild/release.yaml"
  service_account = google_service_account.cloud_build.id
  substitutions   = local.build_substitutions

  approval_config {
    approval_required = true
  }

  github {
    owner = var.github_owner
    name  = var.github_repo

    push {
      tag = "^v.*$"
    }
  }

  depends_on = [
    google_project_service.required,
    google_project_iam_member.build_roles,
    google_service_account_iam_member.build_can_run_as_runtime,
  ]
}
