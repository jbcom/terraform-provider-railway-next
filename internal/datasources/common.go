package datasources

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

func configureClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.Client {
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

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
