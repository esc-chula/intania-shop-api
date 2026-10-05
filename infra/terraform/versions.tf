terraform {
  required_version = ">= 1.16.0"

  # Backend settings are supplied to `terraform init`: the bucket must exist
  # before Terraform can manage any other infrastructure.
  backend "gcs" {}

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 8.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
