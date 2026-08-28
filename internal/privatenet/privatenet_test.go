// SPDX-License-Identifier: MPL-2.0

package privatenet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

// TestReadHandlesAServiceWithNoEndpoint is the regression test for a crash.
//
// **RAILWAY REPORTS "NO PRIVATE ENDPOINT YET" AS NULL, NOT AS AN ERROR.** A
// service being read before Railway has attached it to the private network
// comes back with `privateNetworkEndpoint: null` and no error at all, so the
// obvious code — dereference the result, since `err` was nil — takes the whole
// provider process down with a SIGSEGV.
//
// That happened during a plain `terraform import`, and a crash is the worst
// possible way to report "not yet": the operation that triggered it and every
// other resource in the same graph walk fail together, and the practitioner
// gets a Go stack trace instead of a diagnostic.
func TestReadHandlesAServiceWithNoEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			OperationName string `json:"operationName"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "application/json")

		switch request.OperationName {
		case "GetEnvironmentPrivateNetworks":
			_, _ = io.WriteString(w, `{"data":{"privateNetworks":[{"publicId":"network-fixture","name":"railway","dnsName":"railway","networkId":"1"}]}}`)
		case "GetServicePrivateEndpoint":
			// THE CASE THAT CRASHED: a real network, and no endpoint on it.
			_, _ = io.WriteString(w, `{"data":{"privateNetworkEndpoint":null}}`)
		default:
			_, _ = io.WriteString(w, `{"data":{}}`)
		}
	}))
	defer server.Close()

	railwayClient, err := client.New(client.Config{
		Token:     "fixture-token",
		TokenType: client.TokenTypeAccount,
		Endpoint:  server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	var diagnostics diag.Diagnostics
	endpoint := Read(context.Background(), railwayClient, "environment-fixture", "service-fixture", &diagnostics)

	if diagnostics.HasError() {
		t.Errorf("a service with no endpoint is a normal state, not an error: %v", diagnostics.Errors())
	}
	if endpoint.DNSName != "" || len(endpoint.IPs) != 0 {
		t.Errorf("endpoint = %#v, want the zero value", endpoint)
	}
}

// TestReadHandlesAnEnvironmentWithNoPrivateNetwork covers the other absence.
//
// Private networking can be disabled, and an environment that has it always has
// exactly one network — so anything else means "no address to report" rather
// than a failure worth stopping a read for.
func TestReadHandlesAnEnvironmentWithNoPrivateNetwork(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"privateNetworks":[]}}`)
	}))
	defer server.Close()

	railwayClient, err := client.New(client.Config{
		Token:     "fixture-token",
		TokenType: client.TokenTypeAccount,
		Endpoint:  server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	var diagnostics diag.Diagnostics
	endpoint := Read(context.Background(), railwayClient, "environment-fixture", "service-fixture", &diagnostics)

	if diagnostics.HasError() {
		t.Errorf("no private network is a real configuration: %v", diagnostics.Errors())
	}
	if endpoint.DNSName != "" {
		t.Errorf("endpoint = %#v, want the zero value", endpoint)
	}
}
