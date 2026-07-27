// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPostgresComputedIdentifiersCanConvergeToKnownNull(t *testing.T) {
	t.Parallel()

	state := postgresModel{
		VolumeInstanceID:  types.StringUnknown(),
		ServiceInstanceID: types.StringUnknown(),
		DeploymentID:      types.StringUnknown(),
	}

	resetPostgresComputedIdentifiers(&state)

	for name, value := range map[string]types.String{
		"volume_instance_id":  state.VolumeInstanceID,
		"service_instance_id": state.ServiceInstanceID,
		"deployment_id":       state.DeploymentID,
	} {
		if value.IsUnknown() || !value.IsNull() {
			t.Fatalf("%s = %#v, want known null", name, value)
		}
	}
}
