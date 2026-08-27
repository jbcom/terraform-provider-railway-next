// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestServiceProtocolCreateNormalizesOptionalComputedValues exercises Create
// through Terraform Plugin Protocol v6. In particular, it omits every
// Optional+Computed service setting whose Railway response may be null and
// verifies that no unknown planned value survives into post-apply state.
func TestServiceProtocolCreateNormalizesOptionalComputedValues(t *testing.T) {
	var fixture serviceFixture
	server := httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	defer server.Close()

	config := fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

resource "railway_service" "api" {
  project_id     = "project-fixture"
  environment_id = "environment-fixture"
  name           = "api"
  source_type    = "github"
  repository     = "owner/repository"
  branch         = "master"
  config_path    = "railway.json"
  regions        = { ams = 1 }
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
					resource.TestCheckResourceAttr("railway_service.api", "id", "service-fixture"),
					resource.TestCheckResourceAttr("railway_service.api", "repository", "owner/repository"),
					resource.TestCheckResourceAttr("railway_service.api", "branch", "master"),
					resource.TestCheckResourceAttr("railway_service.api", "config_path", "railway.json"),
					resource.TestCheckResourceAttr("railway_service.api", "regions.ams", "1"),
					resource.TestCheckNoResourceAttr("railway_service.api", "image"),
					resource.TestCheckNoResourceAttr("railway_service.api", "memory_gb"),
					resource.TestCheckNoResourceAttr("railway_service.api", "vcpus"),
					resource.TestCheckNoResourceAttr("railway_service.api", "pre_deploy_command"),
					checkNoUnknownState("railway_service.api"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func checkNoUnknownState(name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		instance, ok := state.RootModule().Resources[name]
		if !ok || instance.Primary == nil {
			return fmt.Errorf("missing state for %s", name)
		}
		for attribute, value := range instance.Primary.Attributes {
			if strings.Contains(strings.ToLower(value), "unknown") {
				return fmt.Errorf("%s.%s remained unknown after apply", name, attribute)
			}
		}
		return nil
	}
}

type serviceFixture struct {
	mu              sync.Mutex
	exists          bool
	connected       bool
	getServiceCalls int
}

func (f *serviceFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
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
	case "CreateService":
		// THE SOURCE IS ATTACHED BY THE CREATE ITSELF now, matching Railway's
		// own API cookbook — `ServiceCreateInput` carries `source` and
		// `branch`, and the provider no longer makes a second `serviceConnect`
		// call. The fixture has to model that, or it reports a service with no
		// source and the plan never converges.
		f.exists = true
		if variables, ok := request.Variables["input"].(map[string]any); ok {
			if _, hasSource := variables["source"]; hasSource {
				f.connected = true
			}
		}
		writeServiceMutation(w, "serviceCreate")
	case "ConnectService":
		f.connected = true
		writeServiceMutation(w, "serviceConnect")
	case "UpdateServiceInstance":
		_, _ = io.WriteString(w, `{"data":{"serviceInstanceUpdate":true}}`)
	case "GetService":
		if !f.exists {
			_, _ = io.WriteString(w, `{"errors":[{"message":"not found","extensions":{"code":"NOT_FOUND"}}]}`)
			return
		}
		f.getServiceCalls++
		repoTriggers := []any{}
		var source any
		if f.connected {
			repoTriggers = []any{map[string]any{
				"node": map[string]any{
					"id": "trigger-fixture", "environmentId": "environment-fixture",
					"branch": "master", "repository": "owner/repository",
				},
			}}
			source = map[string]any{"image": nil, "repo": "owner/repository"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"service": map[string]any{
				"id": "service-fixture", "name": "api", "projectId": "project-fixture",
				"icon": nil, "deletedAt": nil,
				"repoTriggers": map[string]any{"edges": repoTriggers},
			},
			"environment": map[string]any{
				"config": map[string]any{"services": map[string]any{
					"service-fixture": map[string]any{"deploy": map[string]any{
						"multiRegionConfig": map[string]any{
							"ams": map[string]any{"numReplicas": 1},
						},
					}},
				}},
				"serviceInstances": map[string]any{"edges": []any{map[string]any{
					"node": map[string]any{
						"id": "instance-fixture", "environmentId": "environment-fixture",
						"serviceId": "service-fixture", "serviceName": "api",
						"buildCommand": nil, "builder": "RAILPACK", "dockerfilePath": nil,
						"drainingSeconds": nil, "healthcheckPath": nil, "healthcheckTimeout": nil,
						"ipv6EgressEnabled": nil, "numReplicas": nil, "overlapSeconds": nil,
						"preDeployCommand": nil, "railwayConfigFile": "railway.json", "region": nil,
						"restartPolicyMaxRetries": 0, "restartPolicyType": "ON_FAILURE",
						"rootDirectory": nil, "sleepApplication": nil,
						"source":       source,
						"startCommand": nil, "watchPatterns": []any{}, "latestDeployment": nil,
					},
				}}},
			},
			"limitOverride": nil,
		}})
	case "DeleteService":
		f.exists = false
		_, _ = io.WriteString(w, `{"data":{"serviceDelete":true}}`)
	default:
		http.Error(w, "unexpected operation "+request.OperationName, http.StatusBadRequest)
	}
}

func writeServiceMutation(w io.Writer, field string) {
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
		field: map[string]any{
			"id": "service-fixture", "name": "api", "projectId": "project-fixture",
			"icon": nil, "deletedAt": nil,
		},
	}})
}
