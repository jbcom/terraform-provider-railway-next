resource "railway_service" "api" {
  project_id       = railway_project.example.id
  environment_id   = railway_project.example.default_environment_id
  name             = "api"
  source_type      = "github"
  repository       = "acme/example"
  branch           = "main"
  root_directory   = "api"
  healthcheck_path = "/healthz"
  start_command    = "./api"
  regions          = { ams = 1 }
}
