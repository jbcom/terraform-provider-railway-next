resource "railway_variable_collection" "api" {
  project_id     = railway_project.example.id
  environment_id = railway_project.example.default_environment_id
  service_id     = railway_service.api.id

  variables = {
    APP_ENV      = "production"
    DATABASE_URL = railway_postgres.main.references["DATABASE_URL"]
  }
}
