package datasources

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

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

// bigIntValue converts Railway's `BigInt` into the framework's Int64.
//
// **RAILWAY DECLARES `BigInt` AND SENDS A JSON NUMBER**, so the generated type
// is `json.Number` — which accepts either a number or a quoted string and keeps
// the digits exactly as they arrived. Binding it to `string` instead made every
// read fail with `cannot unmarshal number into Go struct field ... of type
// string`.
//
// A value that does not parse is an error rather than a silent zero: "this
// bucket holds nothing" and "the provider could not tell" are different
// answers, and only one of them makes destroying it safe.
func bigIntValue(raw json.Number, attribute string, diagnostics *diag.Diagnostics) types.Int64 {
	if raw == "" {
		return types.Int64Null()
	}
	parsed, err := raw.Int64()
	if err != nil {
		diagnostics.AddError(
			"Unable to read Railway "+attribute,
			"Railway returned "+strconv.Quote(raw.String())+", which is not an integer: "+err.Error(),
		)
		return types.Int64Null()
	}
	return types.Int64Value(parsed)
}
