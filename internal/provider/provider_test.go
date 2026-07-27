// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestProviderSchema(t *testing.T) {
	t.Parallel()

	factory := providerserver.NewProtocol6WithError(New("test")())
	server, err := factory()
	if err != nil {
		t.Fatalf("provider server: %v", err)
	}
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %v", err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema error: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
	if got, want := len(response.ResourceSchemas), 9; got != want {
		t.Errorf("resource schema count = %d, want %d", got, want)
	}
	if got, want := len(response.DataSourceSchemas), 5; got != want {
		t.Errorf("data source schema count = %d, want %d", got, want)
	}
	for _, name := range []string{
		"railway_project",
		"railway_environment",
		"railway_service",
		"railway_volume",
		"railway_variable_collection",
		"railway_secret",
		"railway_service_domain",
		"railway_bucket",
		"railway_postgres",
	} {
		if _, ok := response.ResourceSchemas[name]; !ok {
			t.Errorf("missing resource schema %q", name)
		}
	}
}
