// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestVolumeProtocolCreateWaitsForEnvironmentAttachment exercises Create
// through Terraform Plugin Protocol v6. Railway can return from volumeCreate
// before the corresponding VolumeInstance is visible; the provider must wait
// for that attachment rather than returning an empty Terraform state.
func TestVolumeProtocolCreateWaitsForEnvironmentAttachment(t *testing.T) {
	var fixture volumeFixture
	server := httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	defer server.Close()

	config := fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

resource "railway_volume" "api_data" {
  project_id     = "project-fixture"
  environment_id = "environment-fixture"
  service_id     = "service-fixture"
  name           = "api-data"
  mount_path     = "/data"
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
					resource.TestCheckResourceAttr("railway_volume.api_data", "id", "volume-fixture"),
					resource.TestCheckResourceAttr("railway_volume.api_data", "volume_instance_id", "volume-instance-fixture"),
					resource.TestCheckResourceAttr("railway_volume.api_data", "service_id", "service-fixture"),
					resource.TestCheckResourceAttr("railway_volume.api_data", "mount_path", "/data"),
					resource.TestCheckResourceAttr("railway_volume.api_data", "region", "ams"),
					resource.TestCheckResourceAttr("railway_volume.api_data", "size_mb", "500"),
					resource.TestCheckResourceAttr("railway_volume.api_data", "current_size_mb", "0"),
					resource.TestCheckResourceAttr("railway_volume.api_data", "pending_deletion", "false"),
					checkNoUnknownState("railway_volume.api_data"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})

	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.getVolumeCalls < 2 {
		t.Fatalf("expected Create to poll for the delayed volume instance, got %d reads", fixture.getVolumeCalls)
	}
}

// TestVolumeProtocolCreateFailsWhenEnvironmentAttachmentNeverConverges pins
// the failure mode where Railway exposes the new VolumeInstance before its
// requested service and mount path have converged.
func TestVolumeProtocolCreateFailsWhenEnvironmentAttachmentNeverConverges(t *testing.T) {
	fixture := volumeFixture{neverConverge: true}
	server := httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	defer server.Close()

	config := fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

resource "railway_volume" "api_data" {
  project_id     = "project-fixture"
  environment_id = "environment-fixture"
  service_id     = "service-fixture"
  name           = "api-data"
  mount_path     = "/data"

  timeouts = {
    create = "2s"
  }
}
`, server.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"railway": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexp.MustCompile(`(?s)Unable to confirm Railway volume attachment.*service-fixture.*\/data`),
		}},
	})

	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.getVolumeCalls < 2 {
		t.Fatalf("expected Create to poll until its configured timeout, got %d reads", fixture.getVolumeCalls)
	}
}

type volumeFixture struct {
	mu             sync.Mutex
	exists         bool
	getVolumeCalls int
	neverConverge  bool
}

func (f *volumeFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var request struct {
		OperationName string `json:"operationName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	switch request.OperationName {
	case "CreateVolume":
		f.exists = true
		_, _ = io.WriteString(w, `{"data":{"volumeCreate":{"id":"volume-fixture","name":"api-data","projectId":"project-fixture"}}}`)
	case "GetProjectVolumes":
		if !f.exists {
			_, _ = io.WriteString(w, `{"errors":[{"message":"not found","extensions":{"code":"NOT_FOUND"}}]}`)
			return
		}
		f.getVolumeCalls++
		serviceID := any(nil)
		mountPath := "/tmp"
		if f.getVolumeCalls > 1 && !f.neverConverge {
			serviceID = "service-fixture"
			mountPath = "/data"
		}
		instances := []any{map[string]any{"node": map[string]any{
			"id": "volume-instance-fixture", "volumeId": "volume-fixture",
			"environmentId": "environment-fixture", "serviceId": serviceID,
			"mountPath": mountPath, "region": "ams", "sizeMB": 500,
			"currentSizeMB": 0, "isPendingDeletion": false,
			"deletedAt": nil, "state": "READY",
		}}}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"project": map[string]any{"volumes": map[string]any{"edges": []any{
				map[string]any{"node": map[string]any{
					"id": "volume-fixture", "name": "api-data", "projectId": "project-fixture",
				}},
			}}},
			"environment": map[string]any{"volumeInstances": map[string]any{"edges": instances}},
		}})
	case "DeleteVolume":
		f.exists = false
		_, _ = io.WriteString(w, `{"data":{"volumeDelete":true}}`)
	default:
		http.Error(w, "unexpected operation "+request.OperationName, http.StatusBadRequest)
	}
}
