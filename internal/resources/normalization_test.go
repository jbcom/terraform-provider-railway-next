// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
)

func TestServiceInstanceResponseNormalization(t *testing.T) {
	t.Parallel()

	build := "go build ./cmd/api"
	health := "/healthz"
	timeout := 30
	ipv6 := true
	region := "ams"
	root := "api"
	start := "./api"
	sleep := false
	rawCommands := json.RawMessage(`["migrate","seed"]`)
	remote := railway.ServiceInstanceFields{
		BuildCommand:            &build,
		Builder:                 railway.BuilderRailpack,
		HealthcheckPath:         &health,
		HealthcheckTimeout:      &timeout,
		Ipv6EgressEnabled:       &ipv6,
		NumReplicas:             intPointerValue(2),
		PreDeployCommand:        &rawCommands,
		Region:                  &region,
		RestartPolicyMaxRetries: 10,
		RestartPolicyType:       railway.RestartPolicyTypeOnFailure,
		RootDirectory:           &root,
		SleepApplication:        &sleep,
		StartCommand:            &start,
		WatchPatterns:           []string{"api/**", "go.mod"},
	}
	var state serviceModel
	var diagnostics diag.Diagnostics
	setServiceInstanceState(context.Background(), &state, &remote, &diagnostics)
	if diagnostics.HasError() {
		t.Fatalf("normalization diagnostics: %v", diagnostics)
	}
	if state.Builder.ValueString() != "RAILPACK" ||
		state.HealthcheckPath.ValueString() != "/healthz" ||
		state.ReplicaCount.ValueInt64() != 2 ||
		!state.IPV6EgressEnabled.ValueBool() {
		t.Fatalf("normalized state = %#v", state)
	}
	var commands []string
	diagnostics.Append(state.PreDeployCommand.ElementsAs(context.Background(), &commands, false)...)
	if len(commands) != 2 || commands[0] != "migrate" || commands[1] != "seed" {
		t.Fatalf("pre-deploy commands = %#v", commands)
	}
	var patterns []string
	diagnostics.Append(state.WatchPatterns.ElementsAs(context.Background(), &patterns, false)...)
	if len(patterns) != 2 {
		t.Fatalf("watch patterns = %#v", patterns)
	}
}

func TestMapNormalizationIsKeyStable(t *testing.T) {
	t.Parallel()

	var diagnostics diag.Diagnostics
	value := mapIntToTerraform(context.Background(), map[string]int64{
		"sin": 1,
		"ams": 2,
	}, &diagnostics)
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if value.ElementType(context.Background()) != types.Int64Type {
		t.Fatalf("element type = %s", value.ElementType(context.Background()))
	}
	if value.Elements()["ams"].(types.Int64).ValueInt64() != 2 ||
		value.Elements()["sin"].(types.Int64).ValueInt64() != 1 {
		t.Fatalf("normalized map = %#v", value.Elements())
	}
}

func intPointerValue(value int) *int {
	return &value
}
