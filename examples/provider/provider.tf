terraform {
  required_version = ">= 1.11.0"

  required_providers {
    railway = {
      source  = "micah5/railway-next"
      version = "~> 0.1"
    }
  }
}

provider "railway" {
  token            = var.railway_token
  token_type       = "account"
  graphql_endpoint = "https://backboard.railway.com/graphql/v2"
}

variable "railway_token" {
  type      = string
  sensitive = true
  default   = null
}
