// SPDX-License-Identifier: MPL-2.0

// Package privatenet reads a service's address on Railway's private network.
//
// **THIS IS SHARED BY THE `railway_service` RESOURCE AND ITS DATA SOURCE**, so
// the two cannot drift into disagreeing about what a private address is. A
// practitioner should get the same answer whether they declared the service or
// looked it up.
package privatenet

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

// Endpoint is a service's presence on the private network.
type Endpoint struct {
	DNSName string
	IPs     []string
}

// Read returns the service's private-network endpoint.
//
// **AN ENVIRONMENT WITHOUT A PRIVATE NETWORK IS NOT AN ERROR.** Private
// networking can be disabled, and an environment that has one always has
// exactly one — so anything other than exactly one network means "no address
// to report" rather than a failure worth stopping a read for.
//
// **AND AN EMPTY ADDRESS LIST IS NOT AN ERROR EITHER.** The addresses belong
// to running containers rather than to the service definition, so a service
// that has never deployed has an endpoint with `syncStatus: ACTIVE` and no
// addresses at all. Reporting that as a failure would make every fresh
// environment unreadable.
func Read(
	ctx context.Context,
	railwayClient *client.Client,
	environmentID string,
	serviceID string,
	diagnostics *diag.Diagnostics,
) Endpoint {
	networks, err := railway.GetEnvironmentPrivateNetworks(ctx, railwayClient.GraphQL(), environmentID)
	if err != nil {
		diagnostics.AddError("Unable to read Railway private networks", client.DecodeAPIError(err).Error())
		return Endpoint{}
	}
	if len(networks.PrivateNetworks) != 1 {
		return Endpoint{}
	}

	endpoint, err := railway.GetServicePrivateEndpoint(
		ctx,
		railwayClient.GraphQL(),
		environmentID,
		networks.PrivateNetworks[0].PublicId,
		serviceID,
	)
	if err != nil {
		diagnostics.AddError("Unable to read Railway private endpoint", client.DecodeAPIError(err).Error())
		return Endpoint{}
	}

	// **A SERVICE CAN HAVE NO ENDPOINT, AND RAILWAY REPORTS THAT AS NULL
	// RATHER THAN AS AN ERROR.**
	//
	// Dereferencing it crashed the provider with a SIGSEGV during `import` —
	// the resource is being read before Railway has attached it to the private
	// network, so `privateNetworkEndpoint` is null and `err` is nil.
	//
	// A crash is the worst possible reporting of "not yet": it takes the whole
	// provider process down, so the operation that triggered it and every other
	// resource in the same walk fail together, with a stack trace instead of a
	// diagnostic.
	if endpoint.PrivateNetworkEndpoint == nil {
		return Endpoint{}
	}

	return Endpoint{
		DNSName: endpoint.PrivateNetworkEndpoint.DnsName,
		IPs:     endpoint.PrivateNetworkEndpoint.PrivateIps,
	}
}
