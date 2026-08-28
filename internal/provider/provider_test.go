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
	if got, want := len(response.ResourceSchemas), 10; got != want {
		t.Errorf("resource schema count = %d, want %d", got, want)
	}
	if got, want := len(response.DataSourceSchemas), 6; got != want {
		t.Errorf("data source schema count = %d, want %d", got, want)
	}

	// **BOTH FORMS OF THE BUCKET CREDENTIALS MUST BE REGISTERED.**
	//
	// The ephemeral one is the default and the data source is the escape hatch
	// for arguments Terraform must persist — losing either leaves a
	// practitioner with no way through, which is the situation offering only
	// one of them creates.
	if _, ok := response.DataSourceSchemas["railway_bucket_credentials"]; !ok {
		t.Error("data.railway_bucket_credentials is not registered")
	}
	if got, want := len(response.EphemeralResourceSchemas), 1; got != want {
		t.Errorf("ephemeral resource schema count = %d, want %d", got, want)
	}
	if _, ok := response.EphemeralResourceSchemas["railway_bucket_credentials"]; !ok {
		t.Error("ephemeral.railway_bucket_credentials is not registered")
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
		"railway_deployment_trigger",
	} {
		if _, ok := response.ResourceSchemas[name]; !ok {
			t.Errorf("missing resource schema %q", name)
		}
	}
}
