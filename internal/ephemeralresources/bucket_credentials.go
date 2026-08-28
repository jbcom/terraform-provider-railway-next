// SPDX-License-Identifier: MPL-2.0

// Package ephemeralresources holds values a practitioner needs during an
// operation without writing them to state.
//
// A data source's result is persisted, so modelling Railway's S3 credentials as
// one writes a live secret, in plaintext, into a state file and into every plan
// diff that touches it. Terraform 1.10's ephemeral resources exist for exactly
// this: the value is produced during an operation, passed to whatever consumes
// it, and discarded.
//
// **BOTH FORMS EXIST, AND THIS ONE IS THE DEFAULT RATHER THAN THE ONLY
// OPTION.** `data.railway_bucket_credentials` is the persisted counterpart,
// because an ephemeral value cannot be used where Terraform must persist it —
// and a provider offering only this form would be deciding that a whole class
// of configuration is not allowed to exist. See `internal/bucketcreds`, which
// both share so they cannot drift.
package ephemeralresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/micah5/terraform-provider-railway-next/internal/bucketcreds"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

var (
	_ ephemeral.EphemeralResource              = (*BucketCredentials)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*BucketCredentials)(nil)
)

// BucketCredentials exposes a Railway bucket's real S3 credentials for the
// duration of one Terraform operation.
//
// **PREFER `railway_bucket`'s `references` WHEREVER IT WORKS.** Wiring a
// Railway service to a Railway bucket should use the reference expressions —
// `${{media.SECRET_ACCESS_KEY}}` — which Railway resolves at deploy time so
// no credential is ever in Terraform's hands at all. That is strictly safer
// than handling the real value, however carefully.
//
// This is for the case a reference cannot serve, because the consumer is not a
// Railway service: a provisioner uploading a seed object, an `aws`/`minio`
// provider pointed at the same bucket, or a non-Railway system reading it.
type BucketCredentials struct {
	client *client.Client
}

type bucketCredentialsModel struct {
	BucketID      types.String `tfsdk:"bucket_id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	ProjectID     types.String `tfsdk:"project_id"`

	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	BucketName      types.String `tfsdk:"bucket_name"`
	Endpoint        types.String `tfsdk:"endpoint"`
	Region          types.String `tfsdk:"region"`
	URLStyle        types.String `tfsdk:"url_style"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func NewBucketCredentials() ephemeral.EphemeralResource { return &BucketCredentials{} }

func (e *BucketCredentials) Metadata(
	_ context.Context,
	req ephemeral.MetadataRequest,
	resp *ephemeral.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_bucket_credentials"
}

func (e *BucketCredentials) Schema(
	_ context.Context,
	_ ephemeral.SchemaRequest,
	resp *ephemeral.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "S3-compatible credentials for a Railway bucket, held only for the " +
			"duration of the operation. Prefer `railway_bucket`'s `references` when the consumer " +
			"is a Railway service — those never put a credential in Terraform's hands.",
		Attributes: map[string]schema.Attribute{
			"bucket_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Railway bucket ID.",
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Railway environment ID the bucket is registered in.",
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Railway project ID.",
			},

			"access_key_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "S3 access key ID.",
			},
			// **MARKED SENSITIVE EVEN THOUGH IT IS EPHEMERAL.** Ephemerality
			// keeps it out of state; `Sensitive` keeps it out of CLI output. A
			// value that is never stored can still be printed, and an operator
			// running `terraform apply` in CI would find it in the log.
			"secret_access_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "S3 secret access key.",
			},
			"bucket_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Bucket name as the S3 API expects it.",
			},
			"endpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "S3-compatible endpoint URL.",
			},
			"region": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Bucket region.",
			},
			"url_style": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URL addressing style the endpoint expects, such as `path` or `vhost`.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When these credentials were issued.",
			},
		},
	}
}

func (e *BucketCredentials) Configure(
	_ context.Context,
	req ephemeral.ConfigureRequest,
	resp *ephemeral.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}
	configured, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T. Please report this provider bug.", req.ProviderData),
		)
		return
	}
	e.client = configured
}

// Open fetches the credentials. There is no Renew and no Close: Railway issues
// these as durable credentials rather than as a lease, so there is nothing to
// keep alive and nothing to hand back.
//
// Rotation is a separate, deliberate act — `bucketCredentialsReset` — and not
// something reading them should trigger.
func (e *BucketCredentials) Open(
	ctx context.Context,
	req ephemeral.OpenRequest,
	resp *ephemeral.OpenResponse,
) {
	// **A NIL CLIENT IS A DIAGNOSTIC, NOT A PANIC.**
	//
	// `Configure` returns early when `ProviderData` is nil — which the framework
	// does on purpose during early graph walks, before the provider is
	// configured. Dereferencing the client here crashed the provider process
	// with a SIGSEGV, and a panicking provider tells the practitioner nothing
	// about what is wrong.
	if e.client == nil {
		resp.Diagnostics.AddError(
			"Railway provider is not configured",
			"The provider client was not available when this ephemeral resource was opened. "+
				"Please report this provider bug.",
		)
		return
	}

	var config bucketCredentialsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, diagnostics := bucketcreds.Read(
		ctx,
		e.client,
		config.BucketID.ValueString(),
		config.EnvironmentID.ValueString(),
		config.ProjectID.ValueString(),
	)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.AccessKeyID = types.StringValue(found.AccessKeyId)
	config.SecretAccessKey = types.StringValue(found.SecretAccessKey)
	config.BucketName = types.StringValue(found.BucketName)
	config.Endpoint = types.StringValue(found.Endpoint)
	config.Region = types.StringValue(found.Region)
	config.URLStyle = types.StringValue(found.UrlStyle)
	config.CreatedAt = types.StringValue(found.CreatedAt)

	resp.Diagnostics.Append(resp.Result.Set(ctx, &config)...)
}
