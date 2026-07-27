resource "railway_environment" "staging" {
  project_id           = railway_project.example.id
  name                 = "staging"
  skip_initial_deploys = true
}
