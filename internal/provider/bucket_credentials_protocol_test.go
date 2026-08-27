// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// theSecret is deliberately distinctive so a scan of the state file can prove
// its ABSENCE rather than merely failing to notice it.
const theSecret = "railway-secret-access-key-that-must-never-reach-state"

// TestBucketCredentialsNeverReachState is the property this ephemeral resource
// exists for, driven through the real protocol.
//
// **A DATA SOURCE WOULD HAVE PASSED EVERY OTHER KIND OF TEST.** It would fetch
// the same credentials, expose the same attributes, and behave identically —
// while writing a live secret into the state file in plaintext. The difference
// between the right design and the wrong one is visible only in what is
// persisted, so that is what this asserts.
func TestBucketCredentialsNeverReachState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(serveBucketCredentials))
	defer server.Close()

	config := fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

ephemeral "railway_bucket_credentials" "media" {
  bucket_id      = "bucket-fixture"
  environment_id = "environment-fixture"
  project_id     = "project-fixture"
}
`, server.URL)

	resource.UnitTest(t, resource.TestCase{
		// Ephemeral resources are a Terraform 1.10 feature; an older CLI would
		// fail on the `ephemeral` block itself rather than on anything here.
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"railway": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: func(state *terraform.State) error {
					// The whole state, as Terraform would write it to disk.
					raw, err := json.Marshal(state)
					if err != nil {
						return err
					}
					if strings.Contains(string(raw), theSecret) {
						return fmt.Errorf(
							"the secret access key was written to state; an ephemeral "+
								"resource must not persist:\n%s", raw)
					}
					return nil
				},
			},
		},
	})
}

// TestBucketCredentialsRejectsAmbiguousResults covers the case where Railway
// returns more than one credential set.
//
// Picking the first would make the result depend on an ordering Railway does
// not promise — and silently handing back the wrong key pair is worse than
// failing, because the failure surfaces at whatever tries to use it.
func TestBucketCredentialsRejectsAmbiguousResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(serveTwoCredentialSets))
	defer server.Close()

	config := fmt.Sprintf(`
provider "railway" {
  token            = "fixture-token"
  token_type       = "account"
  graphql_endpoint = %q
}

ephemeral "railway_bucket_credentials" "media" {
  bucket_id      = "bucket-fixture"
  environment_id = "environment-fixture"
  project_id     = "project-fixture"
}
`, server.URL)

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"railway": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile("Ambiguous Railway bucket credentials"),
			},
		},
	})
}

func serveBucketCredentials(w http.ResponseWriter, r *http.Request) {
	writeCredentials(w, r, 1)
}

func serveTwoCredentialSets(w http.ResponseWriter, r *http.Request) {
	writeCredentials(w, r, 2)
}

func writeCredentials(w http.ResponseWriter, r *http.Request, count int) {
	defer r.Body.Close()
	var request struct {
		OperationName string `json:"operationName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if request.OperationName != "GetBucketS3Credentials" {
		_, _ = io.WriteString(w, `{"data":{}}`)
		return
	}

	sets := make([]any, 0, count)
	for range count {
		sets = append(sets, map[string]any{
			"accessKeyId":     "railway-access-key-id",
			"secretAccessKey": theSecret,
			"bucketName":      "media",
			"endpoint":        "https://storage.railway.app",
			"region":          "us-east-1",
			"urlStyle":        "path",
			"createdAt":       "2026-08-27T00:00:00.000Z",
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{"bucketS3Credentials": sets},
	})
}
