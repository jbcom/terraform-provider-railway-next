resource "railway_project" "example" {
  name                     = "terraform-example"
  description              = "Managed by Terraform"
  is_public                = false
  default_environment_name = "production"
  pr_deploys               = false
}
