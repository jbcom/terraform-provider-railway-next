package resources

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

const (
	defaultCreateTimeout = 15 * time.Minute
	defaultReadTimeout   = 2 * time.Minute
	defaultUpdateTimeout = 15 * time.Minute
	defaultDeleteTimeout = 15 * time.Minute
)

type timeoutOperation string

const (
	timeoutCreate timeoutOperation = "create"
	timeoutRead   timeoutOperation = "read"
	timeoutUpdate timeoutOperation = "update"
	timeoutDelete timeoutOperation = "delete"
)

func operationContext(
	ctx context.Context,
	value timeouts.Value,
	operation timeoutOperation,
	diagnostics *diag.Diagnostics,
) (context.Context, context.CancelFunc, bool) {
	var (
		duration time.Duration
		result   diag.Diagnostics
	)
	switch operation {
	case timeoutCreate:
		duration, result = value.Create(ctx, defaultCreateTimeout)
	case timeoutRead:
		duration, result = value.Read(ctx, defaultReadTimeout)
	case timeoutUpdate:
		duration, result = value.Update(ctx, defaultUpdateTimeout)
	case timeoutDelete:
		duration, result = value.Delete(ctx, defaultDeleteTimeout)
	default:
		diagnostics.AddError("Unsupported resource timeout operation", string(operation))
		return ctx, func() {}, false
	}
	diagnostics.Append(result...)
	if diagnostics.HasError() {
		return ctx, func() {}, false
	}
	timed, cancel := context.WithTimeout(ctx, duration)
	return timed, cancel, true
}

func configureClient(
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	configured, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T. Please report this provider bug.", req.ProviderData),
		)
		return nil
	}
	return configured
}

func removeIfNotFound(
	ctx context.Context,
	err error,
	state *resource.ReadResponse,
	summary string,
) bool {
	if client.IsNotFound(err) {
		state.State.RemoveResource(ctx)
		return true
	}
	if err != nil {
		state.Diagnostics.AddError(summary, client.DecodeAPIError(err).Error())
		return true
	}
	return false
}

func importID(
	ctx context.Context,
	id string,
	state *resource.ImportStateResponse,
) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), resource.ImportStateRequest{ID: id}, state)
}

func splitImportID(id string, count int) ([]string, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	parts := strings.Split(id, "/")
	if len(parts) != count {
		diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected %d slash-separated fields, got %d.", count, len(parts)),
		)
		return nil, diagnostics
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			diagnostics.AddError("Invalid import ID", "Import ID fields must not be empty.")
			return nil, diagnostics
		}
	}
	return parts, diagnostics
}

func valueString(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(*value)
}

func stringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := value.ValueString()
	return &result
}

func boolPointer(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := value.ValueBool()
	return &result
}

func intPointer(value types.Int64) *int {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := int(value.ValueInt64())
	return &result
}
