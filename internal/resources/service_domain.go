package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

var (
	_ resource.Resource                = (*ServiceDomain)(nil)
	_ resource.ResourceWithConfigure   = (*ServiceDomain)(nil)
	_ resource.ResourceWithImportState = (*ServiceDomain)(nil)
)

type ServiceDomain struct {
	client *client.Client
}

type serviceDomainModel struct {
	ID            types.String   `tfsdk:"id"`
	ProjectID     types.String   `tfsdk:"project_id"`
	EnvironmentID types.String   `tfsdk:"environment_id"`
	ServiceID     types.String   `tfsdk:"service_id"`
	Kind          types.String   `tfsdk:"kind"`
	Domain        types.String   `tfsdk:"domain"`
	Subdomain     types.String   `tfsdk:"subdomain"`
	Port          types.Int64    `tfsdk:"port"`
	Status        types.String   `tfsdk:"status"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func NewServiceDomain() resource.Resource {
	return &ServiceDomain{}
}

func (r *ServiceDomain) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_domain"
}

func (r *ServiceDomain) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Railway-generated/requested service domain or custom domain.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"project_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("railway", "custom"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Full domain. Required for custom domains; computed for Railway domains.",
			},
			"subdomain": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Requested Railway subdomain. Omit for an automatically generated domain.",
			},
			"port":     schema.Int64Attribute{Optional: true, Computed: true},
			"status":   schema.StringAttribute{Computed: true},
			"timeouts": timeouts.AttributesAll(ctx),
		},
	}
}

func (r *ServiceDomain) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *ServiceDomain) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutCreate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	unlockEnvironment := lockEnvironmentChangeSet(plan.EnvironmentID.ValueString())
	defer unlockEnvironment()
	switch plan.Kind.ValueString() {
	case "custom":
		if plan.Domain.IsNull() || plan.Domain.IsUnknown() || plan.Domain.ValueString() == "" {
			resp.Diagnostics.AddError("Missing custom domain", "domain is required when kind is custom.")
			return
		}
		result, err := railway.CreateCustomDomain(ctx, r.client.GraphQL(), railway.CustomDomainCreateInput{
			ProjectId:     plan.ProjectID.ValueString(),
			EnvironmentId: plan.EnvironmentID.ValueString(),
			ServiceId:     plan.ServiceID.ValueString(),
			Domain:        plan.Domain.ValueString(),
			TargetPort:    intPointer(plan.Port),
		})
		if err != nil {
			resp.Diagnostics.AddError("Unable to create Railway custom domain", client.DecodeAPIError(err).Error())
			return
		}
		plan.ID = types.StringValue(result.CustomDomainCreate.Id)
		plan.Domain = types.StringValue(result.CustomDomainCreate.Domain)
	default:
		result, err := railway.CreateServiceDomain(ctx, r.client.GraphQL(), railway.ServiceDomainCreateInput{
			EnvironmentId: plan.EnvironmentID.ValueString(),
			ServiceId:     plan.ServiceID.ValueString(),
			TargetPort:    intPointer(plan.Port),
		})
		if err != nil {
			resp.Diagnostics.AddError("Unable to create Railway service domain", client.DecodeAPIError(err).Error())
			return
		}
		plan.ID = types.StringValue(result.ServiceDomainCreate.Id)
		plan.Domain = types.StringValue(result.ServiceDomainCreate.Domain)

		// The domain exists now. A domain Terraform has lost track of is
		// serving traffic that no configuration describes — and for a custom
		// domain it also holds the name, so the next apply cannot recreate it.
		if !plan.Subdomain.IsNull() && !plan.Subdomain.IsUnknown() {
			_, err = railway.UpdateServiceDomain(ctx, r.client.GraphQL(), railway.ServiceDomainUpdateInput{
				ServiceDomainId: plan.ID.ValueString(),
				EnvironmentId:   plan.EnvironmentID.ValueString(),
				ServiceId:       plan.ServiceID.ValueString(),
				Domain:          plan.Subdomain.ValueString(),
				TargetPort:      intPointer(plan.Port),
			})
			if err != nil {
				resp.Diagnostics.AddError("Railway domain created but requested subdomain failed", client.DecodeAPIError(err).Error())
				resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
				return
			}
		}
	}
	refreshed := r.refresh(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if !refreshed {
		return
	}
}

func (r *ServiceDomain) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutRead, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	if !r.refresh(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ServiceDomain) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutUpdate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	unlockEnvironment := lockEnvironmentChangeSet(plan.EnvironmentID.ValueString())
	defer unlockEnvironment()
	var err error
	if plan.Kind.ValueString() == "custom" {
		_, err = railway.UpdateCustomDomain(
			ctx,
			r.client.GraphQL(),
			plan.ID.ValueString(),
			plan.EnvironmentID.ValueString(),
			intPointer(plan.Port),
		)
	} else {
		domain := plan.Domain.ValueString()
		if !plan.Subdomain.IsNull() && !plan.Subdomain.IsUnknown() {
			domain = plan.Subdomain.ValueString()
		}
		_, err = railway.UpdateServiceDomain(ctx, r.client.GraphQL(), railway.ServiceDomainUpdateInput{
			ServiceDomainId: plan.ID.ValueString(),
			EnvironmentId:   plan.EnvironmentID.ValueString(),
			ServiceId:       plan.ServiceID.ValueString(),
			Domain:          domain,
			TargetPort:      intPointer(plan.Port),
		})
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Railway service domain", client.DecodeAPIError(err).Error())
		return
	}
	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServiceDomain) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutDelete, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	unlockEnvironment := lockEnvironmentChangeSet(state.EnvironmentID.ValueString())
	defer unlockEnvironment()
	var err error
	if state.Kind.ValueString() == "custom" {
		_, err = railway.DeleteCustomDomain(ctx, r.client.GraphQL(), state.ID.ValueString())
	} else {
		_, err = railway.DeleteServiceDomain(ctx, r.client.GraphQL(), state.ID.ValueString())
	}
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Railway service domain", client.DecodeAPIError(err).Error())
	}
}

func (r *ServiceDomain) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, diagnostics := splitImportID(req.ID, 5)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kind"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), parts[3])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[4])...)
}

func (r *ServiceDomain) refresh(ctx context.Context, state *serviceDomainModel, diagnostics *diag.Diagnostics) bool {
	result, err := railway.ListDomains(
		ctx,
		r.client.GraphQL(),
		state.ProjectID.ValueString(),
		state.EnvironmentID.ValueString(),
		state.ServiceID.ValueString(),
	)
	if client.IsNotFound(err) {
		return false
	}
	if err != nil {
		diagnostics.AddError("Unable to read Railway service domain", client.DecodeAPIError(err).Error())
		return false
	}
	if state.Kind.ValueString() == "custom" {
		for _, domain := range result.Domains.CustomDomains {
			if domain.Id == state.ID.ValueString() && domain.DeletedAt == nil {
				state.Domain = types.StringValue(domain.Domain)
				state.Port = intValue(domain.TargetPort)
				state.Status = types.StringValue(string(domain.SyncStatus))
				return true
			}
		}
		return false
	}
	for _, domain := range result.Domains.ServiceDomains {
		if domain.Id == state.ID.ValueString() && domain.DeletedAt == nil {
			state.Domain = types.StringValue(domain.Domain)
			state.Port = intValue(domain.TargetPort)
			state.Status = types.StringValue(string(domain.SyncStatus))
			return true
		}
	}
	return false
}
