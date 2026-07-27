resource "railway_secret" "api_key" {
  project_id       = railway_project.example.id
  environment_id   = railway_project.example.default_environment_id
  service_id       = railway_service.api.id
  name             = "API_KEY"
  value_wo         = var.api_key
  value_wo_version = 1
}

variable "api_key" {
  type      = string
  sensitive = true
  ephemeral = true
}
