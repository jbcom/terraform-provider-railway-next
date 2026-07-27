resource "railway_bucket" "cache" {
  project_id     = railway_project.example.id
  environment_id = railway_project.example.default_environment_id
  name           = "terraform-example-cache"
  region         = "ams"
}

output "bucket_reference" {
  value = railway_bucket.cache.references["BUCKET"]
}
