// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestEveryResourceKeepsItsIDAcrossAnUpdate is the regression test for the bug
// that cost the most time in this fork.
//
// A Computed `id` with no plan modifier is UNKNOWN in the plan of any update,
// so `plan.ID.ValueString()` is `""` and the provider asks Railway to modify an
// object with no id. Railway answers:
//
//	Not Authorized
//
// which names neither the id nor the resource, so every reader goes looking at
// their token. The token was fine. The same mutation issued by hand succeeded
// every time — the only wrong thing was an empty string in a variable no error
// message mentions.
//
// **THIS ASSERTS THE PROPERTY ACROSS THE WHOLE REGISTRY, NOT ONE RESOURCE.**
// All eight resources with an `id` had it wrong at once, so a test naming
// `railway_service` would have proved the fix and still let the next resource
// ship broken.
//
// It lives in `internal/provider` rather than beside the schemas because the
// provider's `Resources()` is the ONE list of what this provider exposes.
// Enumerating them again in the test would be a second list to keep in step,
// and a resource missing from it would silently skip the check — which is the
// same class of drift that made the hand-written unknown-resolution list miss
// four fields.
func TestEveryResourceKeepsItsIDAcrossAnUpdate(t *testing.T) {
	t.Parallel()

	provider := &RailwayProvider{}
	factories := provider.Resources(context.Background())

	if len(factories) == 0 {
		t.Fatal("provider registered no resources, so this test proves nothing")
	}

	for _, factory := range factories {
		res := factory()

		var metadata resource.MetadataResponse
		res.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "railway"}, &metadata)

		var schemaResponse resource.SchemaResponse
		res.Schema(context.Background(), resource.SchemaRequest{}, &schemaResponse)

		t.Run(metadata.TypeName, func(t *testing.T) {
			t.Parallel()

			attribute, ok := schemaResponse.Schema.Attributes["id"]
			if !ok {
				// Not every resource must have an `id` — but one that does must
				// keep it.
				t.Skip("no id attribute")
			}

			stringAttribute, ok := attribute.(schema.StringAttribute)
			if !ok {
				t.Fatalf("id is %T, want schema.StringAttribute", attribute)
			}

			if !stringAttribute.Computed {
				t.Error("id is not Computed")
			}

			// Identified by description rather than by type, because
			// `UseStateForUnknown` returns an unexported type that cannot be
			// named from outside the framework.
			for _, modifier := range stringAttribute.PlanModifiers {
				if strings.Contains(modifier.Description(context.Background()), "state") {
					return
				}
			}

			t.Error("id has no UseStateForUnknown plan modifier, so it is unknown " +
				"during an update and the provider sends an empty id to Railway")
		})
	}
}
