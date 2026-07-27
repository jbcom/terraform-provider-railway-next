terraform {
  required_version = ">= 1.11.0"

  required_providers {
    railway = {
      source  = "micah5/railway-next"
      version = "~> 0.1"
    }
  }
}

provider "railway" {
  token      = var.railway_token
  token_type = var.railway_token_type
}

resource "railway_project" "main" {
  name                     = var.project_name
  description              = "API, UI, PostgreSQL, and bucket managed by Terraform"
  is_public                = false
  workspace_id             = var.workspace_id
  default_environment_name = "production"
  pr_deploys               = false
}

resource "railway_postgres" "main" {
  project_id     = railway_project.main.id
  environment_id = railway_project.main.default_environment_id
  name           = "Postgres"
  version        = "18"
  region         = "ams"
}

resource "railway_bucket" "cache" {
  project_id     = railway_project.main.id
  environment_id = railway_project.main.default_environment_id
  name           = "langgraph-pa-cache"
  region         = "ams"
}

resource "railway_service" "api" {
  project_id     = railway_project.main.id
  environment_id = railway_project.main.default_environment_id
  name           = "api"
  source_type    = "github"
  repository     = var.github_repository
  branch         = var.github_branch

  root_directory             = "api"
  healthcheck_path           = "/healthz"
  healthcheck_timeout        = 30
  restart_policy_type        = "ON_FAILURE"
  restart_policy_max_retries = 10
  regions = {
    ams = 1
  }
}

resource "railway_volume" "api" {
  project_id     = railway_project.main.id
  environment_id = railway_project.main.default_environment_id
  service_id     = railway_service.api.id
  name           = "api-data"
  mount_path     = "/data"
  region         = "ams"
}

resource "railway_variable_collection" "api" {
  project_id     = railway_project.main.id
  environment_id = railway_project.main.default_environment_id
  service_id     = railway_service.api.id

  variables = {
    APP_ENV                  = "production"
    DATABASE_URL             = railway_postgres.main.references["DATABASE_URL"]
    BUCKET                   = railway_bucket.cache.references["BUCKET"]
    BUCKET_ENDPOINT          = railway_bucket.cache.references["ENDPOINT"]
    BUCKET_ACCESS_KEY_ID     = railway_bucket.cache.references["ACCESS_KEY_ID"]
    BUCKET_SECRET_ACCESS_KEY = railway_bucket.cache.references["SECRET_ACCESS_KEY"]
    BUCKET_REGION            = railway_bucket.cache.references["REGION"]
  }
}

resource "railway_service" "ui" {
  project_id       = railway_project.main.id
  environment_id   = railway_project.main.default_environment_id
  name             = "ui"
  source_type      = "github"
  repository       = var.github_repository
  branch           = var.github_branch
  root_directory   = "ui"
  healthcheck_path = "/healthz"
  regions          = { ams = 1 }
}

resource "railway_service_domain" "ui" {
  project_id     = railway_project.main.id
  environment_id = railway_project.main.default_environment_id
  service_id     = railway_service.ui.id
  kind           = "railway"
}

output "ui_url" {
  value = "https://${railway_service_domain.ui.domain}"
}

output "database_url_reference" {
  value = railway_postgres.main.references["DATABASE_URL"]
}

output "bucket_references" {
  value = railway_bucket.cache.references
}
