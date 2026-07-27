resource "railway_volume" "api_data" {
  project_id     = railway_project.example.id
  environment_id = railway_project.example.default_environment_id
  service_id     = railway_service.api.id
  name           = "api-data"
  mount_path     = "/data"
  region         = "ams"
}
