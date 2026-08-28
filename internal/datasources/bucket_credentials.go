package datasources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/micah5/terraform-provider-railway-next/internal/bucketcreds"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

// BucketCredentials is the PERSISTED counterpart to
// `ephemeral.railway_bucket_credentials`.
//
// **THE PROVIDER DOES NOT GET TO DECIDE THIS FOR PEOPLE.** Offering only the
// ephemeral form looks like the safe choice and is really a refusal: an
// ephemeral value cannot be used where Terraform must persist it, so a
// provider that offers nothing else has decided that a whole class of
// configuration is not allowed to exist. `doppler_secret.value` is exactly
// that case — it is a normal required attribute, so Terraform rejects an
// ephemeral value for it outright:
//
//	Error: Invalid use of ephemeral value
//	Ephemeral values are not valid for "value", because it is not a
//	write-only attribute and must be persisted to state.
//
// The practitioner facing that has a real problem and no way through, for a
// reason that is about this provider's opinions rather than about their
// system.
//
// **SO BOTH EXIST, AND THE DIFFERENCE IS STATED RATHER THAN IMPLIED.** The
// ephemeral resource is the better default and the documentation says so. This
// one is for when the value must be persisted — and it is honest about what
// that costs, in its description, in its `Sensitive` markings, and in the
// warning it raises on every read.
//
// The one thing it does NOT do is pretend. A data source writes to state; a
// practitioner choosing it should know that from the provider, not discover it
// from a state file.
type BucketCredentials struct{ client *client.Client }

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

func NewBucketCredentials() datasource.DataSource { return &BucketCredentials{} }

func (d *BucketCredentials) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_bucket_credentials"
}

func (d *BucketCredentials) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "S3-compatible credentials for a Railway bucket, **written to Terraform state**.\n\n" +
			"Prefer `ephemeral.railway_bucket_credentials`, which fetches the same values without " +
			"persisting them. Use this one only where Terraform must persist the value — for " +
			"example an argument that is not write-only, which rejects an ephemeral value outright. " +
			"Treat the state file as a secret when you do.",
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
			// **SENSITIVE KEEPS IT OUT OF CLI OUTPUT; NOTHING KEEPS IT OUT OF
			// STATE.** That is the whole trade this data source makes, and the
			// marking should not be read as making it safe.
			"secret_access_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "S3 secret access key. **Written to Terraform state in plaintext.**",
			},
			"bucket_name": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Bucket name as the S3 API expects it. This is NOT the Railway " +
					"bucket name — Railway suffixes it, so anything talking S3 must read it from here.",
			},
			"endpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "S3-compatible endpoint URL.",
			},
			"region": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Bucket region as the S3 API reports it, which may be `auto`.",
			},
			"url_style": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URL addressing style the endpoint expects, such as `path` or `virtual-host`.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When these credentials were issued.",
			},
		},
	}
}

func (d *BucketCredentials) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.client = configureClient(req, resp)
}

func (d *BucketCredentials) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	if d.client == nil {
		resp.Diagnostics.AddError(
			"Railway provider is not configured",
			"The provider client was not available when this data source was read. "+
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
		d.client,
		config.BucketID.ValueString(),
		config.EnvironmentID.ValueString(),
		config.ProjectID.ValueString(),
	)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}

	// **SAID EVERY TIME, NOT ONCE IN THE DOCUMENTATION.**
	//
	// The cost of this data source is invisible at the point of use: the
	// configuration looks like any other lookup, and the secret lands in state
	// silently. A warning on each read is the only place a practitioner
	// reliably meets that fact — and it names the alternative, so the warning
	// is actionable rather than merely disapproving.
	resp.Diagnostics.AddWarning(
		"Railway bucket credentials written to Terraform state",
		"`data.railway_bucket_credentials` persists a live S3 secret key in plaintext in "+
			"Terraform state. Anyone who can read the state file can read the bucket.\n\n"+
			"Where the value does not have to be persisted, `ephemeral.railway_bucket_credentials` "+
			"takes the same arguments and returns the same attributes without writing them. "+
			"If you are using this because an argument rejected an ephemeral value, that argument "+
			"would persist the secret regardless — protect the state file accordingly.",
	)

	config.AccessKeyID = types.StringValue(found.AccessKeyId)
	config.SecretAccessKey = types.StringValue(found.SecretAccessKey)
	config.BucketName = types.StringValue(found.BucketName)
	config.Endpoint = types.StringValue(found.Endpoint)
	config.Region = types.StringValue(found.Region)
	config.URLStyle = types.StringValue(found.UrlStyle)
	config.CreatedAt = types.StringValue(found.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
