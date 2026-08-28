# THIS WRITES A LIVE SECRET TO TERRAFORM STATE. Reach for
# `ephemeral.railway_bucket_credentials` first — it takes the same arguments and
# returns the same attributes without persisting them.
#
# Use this form when Terraform MUST persist the value, which happens whenever
# the consuming argument is not write-only. Terraform rejects an ephemeral value
# there outright:
#
#   Error: Invalid use of ephemeral value
#   Ephemeral values are not valid for "value", because it is not a
#   write-only attribute and must be persisted to state.
#
# When that is the situation, the secret is going into state either way. What
# this data source changes is that you chose it knowingly.
data "railway_bucket_credentials" "media" {
  bucket_id      = railway_bucket.media.id
  environment_id = railway_bucket.media.environment_id
  project_id     = railway_bucket.media.project_id
}

# Storing the key in a secrets manager whose provider has no write-only
# argument yet is the case this exists for.
resource "doppler_secret" "bucket_key" {
  project = "example"
  config  = "uat"
  name    = "AWS_SECRET_ACCESS_KEY"
  value   = data.railway_bucket_credentials.media.secret_access_key
}

# MARK ANY OUTPUT SENSITIVE. Without this Terraform prints the key to the
# console and into CI logs on every apply — and it refuses the plan rather than
# doing so silently, which is the one part of this that fails loudly.
output "bucket_access_key_id" {
  value     = data.railway_bucket_credentials.media.access_key_id
  sensitive = true
}
