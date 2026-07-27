resource "railway_service_domain" "api" {
  project_id     = railway_project.example.id
  environment_id = railway_project.example.default_environment_id
  service_id     = railway_service.api.id
  kind           = "railway"
  subdomain      = "terraform-example-api"
  port           = 8080
}
