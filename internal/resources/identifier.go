// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// idAttribute is the `id` attribute every resource in this provider exposes.
//
// **WITHOUT `UseStateForUnknown` AN UPDATE TALKS TO THE API ABOUT AN OBJECT
// WITH NO ID, AND THAT WAS TRUE OF ALL EIGHT RESOURCES HERE.**
//
// A Computed attribute with no plan modifier is UNKNOWN in the plan whenever
// anything else on the resource changes — Terraform cannot assume a computed
// value survives a change it has not seen the provider make. `id` is the one
// computed value that always does survive: Railway does not re-issue an id
// because a service was renamed or its source re-pointed.
//
// So `plan.ID.ValueString()` in an `Update` returned `""`, and the provider
// asked Railway to update the service instance of service `""`. Railway's
// answer to that is:
//
//	Not Authorized
//
// which sends every reader looking at their token. The token was fine. There
// was no service to be authorized against, because the id never left state.
//
// The same mutation issued by hand succeeded every time, with the same token,
// which is the detail that makes this so slow to find: the payload was right,
// the credentials were right, and the only wrong thing was an empty string in a
// variable that no error message mentions.
//
// `UseStateForUnknown` is the framework's answer — it copies the prior state
// value into the plan instead of marking it unknown. It is correct precisely
// BECAUSE the id is immutable for the life of the resource; anything Railway
// genuinely recomputes must not use this.
//
// Resources whose id is not stable across an update must not call this.
func idAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}
