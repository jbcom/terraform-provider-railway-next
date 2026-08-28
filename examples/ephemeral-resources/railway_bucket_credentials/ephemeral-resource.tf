# PREFER `references` WHEN THE CONSUMER RUNS INSIDE RAILWAY.
#
# A Railway service reading a Railway bucket should use the reference
# expressions, which Railway resolves at deploy time — no credential is ever in
# Terraform's hands:
#
#   value = railway_bucket.media.references["SECRET_ACCESS_KEY"]
#
# This ephemeral resource is for the case a reference cannot serve, because the
# consumer is NOT a Railway service.
ephemeral "railway_bucket_credentials" "media" {
  bucket_id      = railway_bucket.media.id
  environment_id = railway_bucket.media.environment_id
  project_id     = railway_bucket.media.project_id
}

# Configuring another provider against the same bucket is the case that needs
# the real values. `ephemeral` keeps them out of state and out of the plan file.
provider "aws" {
  alias      = "railway_bucket"
  region     = ephemeral.railway_bucket_credentials.media.region
  access_key = ephemeral.railway_bucket_credentials.media.access_key_id
  secret_key = ephemeral.railway_bucket_credentials.media.secret_access_key

  s3_use_path_style           = ephemeral.railway_bucket_credentials.media.url_style == "path"
  skip_credentials_validation = true
  skip_requesting_account_id  = true

  endpoints {
    s3 = ephemeral.railway_bucket_credentials.media.endpoint
  }
}

# NOTE: an ephemeral value cannot be used in `output`, and that restriction is
# the point — Terraform enforces what the type promises rather than trusting
# every consumer to remember.
