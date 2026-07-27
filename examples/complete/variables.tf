variable "railway_token" {
  type      = string
  sensitive = true
  default   = null
}

variable "railway_token_type" {
  type    = string
  default = "account"

  validation {
    condition     = contains(["account", "workspace", "project"], var.railway_token_type)
    error_message = "railway_token_type must be account, workspace, or project."
  }
}

variable "workspace_id" {
  type     = string
  nullable = true
  default  = null
}

variable "project_name" {
  type    = string
  default = "terraform-complete-example"
}

variable "github_repository" {
  type        = string
  description = "GitHub repository in owner/name form containing api/ and ui/."
}

variable "github_branch" {
  type    = string
  default = "main"
}
