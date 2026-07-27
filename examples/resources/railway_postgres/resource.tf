resource "railway_postgres" "main" {
  project_id     = railway_project.example.id
  environment_id = railway_project.example.default_environment_id
  name           = "Postgres"
  version        = "18"
  region         = "ams"
}

output "database_url_reference" {
  value = railway_postgres.main.references["DATABASE_URL"]
}
