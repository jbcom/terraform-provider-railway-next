// SPDX-License-Identifier: MPL-2.0

// Package bucketcreds holds the one fetch of a Railway bucket's S3
// credentials, shared by the two forms the provider offers.
//
// **THE PROVIDER OFFERS BOTH ON PURPOSE.**
// `ephemeral.railway_bucket_credentials` is the better default — it never
// persists. `data.railway_bucket_credentials` exists because an ephemeral
// value cannot be used where Terraform must persist it, and a provider that
// offered only the ephemeral form would be deciding that a whole class of
// configuration is not allowed to exist.
//
// They share this function so they cannot drift into disagreeing about what a
// credential lookup returns or when it is ambiguous. A difference between them
// should be a difference in PERSISTENCE and nothing else.
package bucketcreds

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

// Read is shared with the ephemeral resource so the two forms
// cannot drift into disagreeing about what they return.
//
// The ambiguity check lives here for the same reason: Railway returns a LIST,
// and picking one silently would make the result depend on an ordering the API
// does not promise.
func Read(
	ctx context.Context,
	railwayClient *client.Client,
	bucketID string,
	environmentID string,
	projectID string,
) (*railway.GetBucketS3CredentialsBucketS3CredentialsBucketS3CompatibleCredentials, diag.Diagnostics) {
	var diagnostics diag.Diagnostics

	result, err := railway.GetBucketS3Credentials(
		ctx,
		railwayClient.GraphQL(),
		bucketID,
		environmentID,
		projectID,
	)
	if err != nil {
		diagnostics.AddError(
			"Unable to read Railway bucket credentials",
			client.DecodeAPIError(err).Error(),
		)
		return nil, diagnostics
	}

	credentials := result.BucketS3Credentials
	if len(credentials) != 1 {
		diagnostics.AddError(
			"Ambiguous Railway bucket credentials",
			fmt.Sprintf(
				"Railway returned %d credential sets for this bucket; expected exactly one. "+
					"Reset the bucket's credentials in Railway so a single set remains.",
				len(credentials),
			),
		)
		return nil, diagnostics
	}

	return &credentials[0], diagnostics
}
