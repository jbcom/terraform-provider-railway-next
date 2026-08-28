// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestDeploymentTriggerImportRoundTrips is the property an import has to have
// and this resource did not.
//
// **AN ATTRIBUTE THE READ NEVER FILLS IS AN ATTRIBUTE THE PLAN ALWAYS SEES AS
// CHANGING.** `provider_name` forces replacement, so the first version — whose
// `Read` restored only `branch` and `repository` — turned every imported
// trigger into a plan that destroyed and recreated it. Against live Railway
// that read `Plan: 1 to add, 0 to change, 1 to destroy` immediately after a
// successful import, which is the shape of a resource that cannot be adopted.
//
// The fix was in the GraphQL selection rather than in Go: the service query did
// not ask for `provider` or `checkSuites`, so `Read` had nothing to restore
// them from. This drives the whole cycle — create, import, plan — because that
// is the only way the defect is visible.
func TestDeploymentTriggerImportRoundTrips(t *testing.T) {
	fixture := &deploymentTriggerFixture{}
	server := httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	defer server.Close()

	config := fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

resource "railway_deployment_trigger" "web" {
  project_id     = "project-fixture"
  environment_id = "environment-fixture"
  service_id     = "service-fixture"
  repository     = "owner/repository"
  branch         = "uat"
}
`, server.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"railway": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("railway_deployment_trigger.web", "id", "trigger-fixture"),
					resource.TestCheckResourceAttr("railway_deployment_trigger.web", "branch", "uat"),
					// Defaulted rather than configured, so a read that does not
					// restore them is a read that loses them.
					resource.TestCheckResourceAttr("railway_deployment_trigger.web", "provider_name", "github"),
					resource.TestCheckResourceAttr("railway_deployment_trigger.web", "check_suites", "true"),
				),
			},
			{
				// **THE ASSERTION.** `ImportStateVerify` compares the imported
				// state against the state the create produced, attribute by
				// attribute. A `provider_name` the read leaves unset fails here
				// rather than in somebody's plan.
				ResourceName:  "railway_deployment_trigger.web",
				ImportState:   true,
				ImportStateId: "project-fixture/environment-fixture/service-fixture/trigger-fixture",
				// `timeouts` is a configuration-only block with nothing to read
				// back, so it is absent from an imported resource by design.
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"timeouts"},
				ImportStateVerifyIdentifierAttribute: "id",
			},
		},
	})
}

// TestDeploymentTriggerImportedCheckSuitesUpdatesInPlace reproduces the live
// Objexiv recovery path: import a trigger whose check suites are disabled, then
// enable them in configuration. provider_name must be restored by Read so the
// plan reaches Update instead of replacing the trigger.
func TestDeploymentTriggerImportedCheckSuitesUpdatesInPlace(t *testing.T) {
	fixture := &deploymentTriggerFixture{
		exists:  true,
		id:      "trigger-fixture",
		branch:  "uat",
		checked: false,
	}
	server := httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	defer server.Close()

	config := func(checkSuites bool) string {
		return fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

resource "railway_deployment_trigger" "web" {
  project_id     = "project-fixture"
  environment_id = "environment-fixture"
  service_id     = "service-fixture"
  repository     = "owner/repository"
  branch         = "uat"
  check_suites   = %t
}
`, server.URL, checkSuites)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"railway": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				ResourceName:       "railway_deployment_trigger.web",
				ImportState:        true,
				ImportStateId:      "project-fixture/environment-fixture/service-fixture/trigger-fixture",
				ImportStatePersist: true,
				Config:             config(false),
			},
			{
				Config: config(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("railway_deployment_trigger.web", "id", "trigger-fixture"),
					resource.TestCheckResourceAttr("railway_deployment_trigger.web", "provider_name", "github"),
					resource.TestCheckResourceAttr("railway_deployment_trigger.web", "check_suites", "true"),
					fixture.checkMutationCounts(0, 1, 0),
				),
			},
		},
	})
}

// TestDeploymentTriggerReadAdoptsUniqueSemanticMatch covers Railway replacing
// a trigger id behind Terraform. The new id is adopted only because the
// service-scoped response has one matching environment/repository/branch.
func TestDeploymentTriggerReadAdoptsUniqueSemanticMatch(t *testing.T) {
	fixture := &deploymentTriggerFixture{}
	server := httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	defer server.Close()

	config := fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

resource "railway_deployment_trigger" "web" {
  project_id     = "project-fixture"
  environment_id = "environment-fixture"
  service_id     = "service-fixture"
  repository     = "owner/repository"
  branch         = "uat"
}
`, server.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"railway": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fixture.replaceID("replacement-trigger-fixture")
				},
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("railway_deployment_trigger.web", "id", "replacement-trigger-fixture"),
					fixture.checkMutationCounts(1, 0, 0),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// deploymentTriggerFixture is a Railway that stores one trigger and reports it
// through the service, which is the only way a trigger can be read.
type deploymentTriggerFixture struct {
	mu          sync.Mutex
	exists      bool
	id          string
	branch      string
	checked     bool
	createCalls int
	updateCalls int
	deleteCalls int
}

func (f *deploymentTriggerFixture) replaceID(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.id = id
}

func (f *deploymentTriggerFixture) checkMutationCounts(
	creates int,
	updates int,
	deletes int,
) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.createCalls != creates || f.updateCalls != updates || f.deleteCalls != deletes {
			return fmt.Errorf(
				"unexpected deployment trigger mutations: create=%d update=%d delete=%d; want %d/%d/%d",
				f.createCalls,
				f.updateCalls,
				f.deleteCalls,
				creates,
				updates,
				deletes,
			)
		}
		return nil
	}
}

func (f *deploymentTriggerFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var request struct {
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	trigger := func() map[string]any {
		id := f.id
		if id == "" {
			id = "trigger-fixture"
		}
		return map[string]any{
			"id":            id,
			"branch":        f.branch,
			"repository":    "owner/repository",
			"provider":      "github",
			"projectId":     "project-fixture",
			"environmentId": "environment-fixture",
			"serviceId":     "service-fixture",
			"checkSuites":   f.checked,
		}
	}

	switch request.OperationName {
	case "CreateDeploymentTrigger":
		input, _ := request.Variables["input"].(map[string]any)
		f.exists = true
		f.createCalls++
		if f.id == "" {
			f.id = "trigger-fixture"
		}
		f.branch, _ = input["branch"].(string)
		f.checked, _ = input["checkSuites"].(bool)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"deploymentTriggerCreate": trigger()},
		})

	case "GetEnvironmentPrivateNetworks":
		// **THE FIXTURE REPORTS NO PRIVATE NETWORK**, which is a real state:
		// private networking can be disabled. `privatenet.Read` treats
		// anything other than exactly one network as "no address to report"
		// rather than an error, so this exercises that path.
		_, _ = io.WriteString(w, `{"data":{"privateNetworks":[]}}`)

	case "GetService":
		edges := []any{}
		if f.exists {
			edges = append(edges, map[string]any{"node": trigger()})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"service": map[string]any{
				"id": "service-fixture", "name": "web", "projectId": "project-fixture",
				"icon": nil, "deletedAt": nil,
				"repoTriggers": map[string]any{"edges": edges},
			},
			"environment": map[string]any{
				"config":           map[string]any{"services": map[string]any{}},
				"serviceInstances": map[string]any{"edges": []any{}},
			},
		}})

	case "UpdateDeploymentTrigger":
		input, _ := request.Variables["input"].(map[string]any)
		f.updateCalls++
		if branch, ok := input["branch"].(string); ok {
			f.branch = branch
		}
		if checked, ok := input["checkSuites"].(bool); ok {
			f.checked = checked
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"deploymentTriggerUpdate": trigger()},
		})

	case "DeleteDeploymentTrigger":
		f.exists = false
		f.deleteCalls++
		_, _ = io.WriteString(w, `{"data":{"deploymentTriggerDelete":true}}`)

	default:
		_, _ = io.WriteString(w, `{"data":{}}`)
	}
}
