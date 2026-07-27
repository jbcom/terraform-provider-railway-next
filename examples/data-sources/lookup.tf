data "railway_project" "main" {
  id = var.project_id
}

data "railway_environment" "production" {
  project_id = data.railway_project.main.id
  name       = "production"
}

data "railway_service" "api" {
  project_id     = data.railway_project.main.id
  environment_id = data.railway_environment.production.id
  name           = "api"
}

data "railway_bucket" "cache" {
  project_id     = data.railway_project.main.id
  environment_id = data.railway_environment.production.id
  name           = "langgraph-pa-cache"
}

data "railway_deployment_status" "api" {
  id = data.railway_service.api.latest_deployment_id
}

variable "project_id" {
  type = string
}
