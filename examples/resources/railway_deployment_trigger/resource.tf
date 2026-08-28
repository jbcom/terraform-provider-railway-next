# **A SERVICE WITH A REPOSITORY SOURCE DOES NOT DEPLOY BY ITSELF.**
#
# `railway_service`'s `repository` and `branch` say what the service is made
# of. They do not subscribe it to anything. Without a trigger the service sits
# at no deployment forever, while the UI and the API both show it as correctly
# configured — because it is, apart from this.
resource "railway_service" "web" {
  project_id     = railway_environment.uat.project_id
  environment_id = railway_environment.uat.id
  name           = "web"

  source_type = "github"
  repository  = "example/app"
  branch      = "uat"
}

resource "railway_deployment_trigger" "web" {
  project_id     = railway_service.web.project_id
  environment_id = railway_service.web.environment_id
  service_id     = railway_service.web.id

  # MUST MATCH THE SERVICE'S SOURCE. A trigger for a different repository
  # deploys commits the service was not built from.
  repository = railway_service.web.repository
  branch     = railway_service.web.branch

  # DEFAULTS TO TRUE, which is the safer direction: a red build should not
  # reach an environment. Set false where the repository has no checks at all,
  # since a trigger waiting on check suites that never run never deploys.
  check_suites = true
}

# NO TRIGGER IS ALSO A VALID CONFIGURATION, and it is why this is a separate
# resource rather than a field on the service. A service deployed only by CI or
# by hand has a source and deliberately no trigger; folding the two together
# would make that unexpressible.
