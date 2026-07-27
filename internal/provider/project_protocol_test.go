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
)

// TestProjectProtocolLifecycle exercises plan, state refresh, a second empty
// plan, import, and destroy through Terraform Plugin Protocol v6. Railway is
// replaced with a deterministic GraphQL fixture server, so this test is safe
// and requires no credentials.
func TestProjectProtocolLifecycle(t *testing.T) {
	var fixture projectFixture
	server := httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	defer server.Close()

	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"railway": providerserver.NewProtocol6WithError(New("test")()),
	}
	config := fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

resource "railway_project" "test" {
  name                         = "tf-unit-project"
  description                  = "protocol lifecycle"
  is_public                    = false
  default_environment_name     = "production"
  pr_deploys                   = false
  bot_pr_environments          = false
  focused_pr_environments      = false
}
`, server.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("railway_project.test", "id", "project-fixture"),
					resource.TestCheckResourceAttr("railway_project.test", "default_environment_id", "environment-fixture"),
					resource.TestCheckResourceAttr("railway_project.test", "workspace_id", "workspace-fixture"),
					resource.TestCheckResourceAttr("railway_project.test", "focused_pr_environments", "false"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
			{
				ResourceName:      "railway_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:   config,
				PlanOnly: true,
			},
			{
				PreConfig: func() {
					fixture.setExists(false)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

type projectFixture struct {
	mu          sync.Mutex
	exists      bool
	name        string
	description string
	prDeploys   bool
	focusedPR   bool
}

func (f *projectFixture) setExists(exists bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exists = exists
}

func (f *projectFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
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

	switch request.OperationName {
	case "CreateProject":
		input, _ := request.Variables["input"].(map[string]any)
		f.exists = true
		f.name, _ = input["name"].(string)
		f.description, _ = input["description"].(string)
		// Railway chooses the workspace and may retain its PR defaults even
		// when the create input includes different values.
		f.prDeploys = true
		f.focusedPR = true
		f.writeProject(w, "projectCreate")
	case "GetProject":
		if !f.exists {
			_, _ = io.WriteString(w, `{"errors":[{"message":"not found","extensions":{"code":"NOT_FOUND"}}]}`)
			return
		}
		f.writeProject(w, "project")
	case "UpdateProject":
		input, _ := request.Variables["input"].(map[string]any)
		if name, ok := input["name"].(string); ok {
			f.name = name
		}
		if description, ok := input["description"].(string); ok {
			f.description = description
		}
		if prDeploys, ok := input["prDeploys"].(bool); ok {
			f.prDeploys = prDeploys
		}
		if focusedPR, ok := input["focusedPrEnvironments"].(bool); ok {
			f.focusedPR = focusedPR
		}
		f.writeProject(w, "projectUpdate")
	case "DeleteProject":
		f.exists = false
		_, _ = io.WriteString(w, `{"data":{"projectDelete":true}}`)
	default:
		http.Error(w, "unexpected operation "+request.OperationName, http.StatusBadRequest)
	}
}

func (f *projectFixture) writeProject(w http.ResponseWriter, field string) {
	response := map[string]any{
		"data": map[string]any{
			field: map[string]any{
				"id":                    "project-fixture",
				"name":                  f.name,
				"description":           f.description,
				"isPublic":              false,
				"workspaceId":           "workspace-fixture",
				"baseEnvironmentId":     "environment-fixture",
				"primaryEnvironmentId":  "environment-fixture",
				"prDeploys":             f.prDeploys,
				"botPrEnvironments":     false,
				"focusedPrEnvironments": f.focusedPR,
				"environments": map[string]any{
					"edges": []any{
						map[string]any{"node": map[string]any{
							"id": "environment-fixture", "name": "production", "isEphemeral": false,
						}},
					},
				},
			},
		},
	}
	_ = json.NewEncoder(w).Encode(response)
}
