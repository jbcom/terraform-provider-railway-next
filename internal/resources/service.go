package resources

import (
	"context"
	"encoding/json"
	"fmt"

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
	_ resource.Resource                = (*Service)(nil)
	_ resource.ResourceWithConfigure   = (*Service)(nil)
	_ resource.ResourceWithImportState = (*Service)(nil)
)

type Service struct {
	client *client.Client
}

type serviceModel struct {
	ID                     types.String   `tfsdk:"id"`
	ProjectID              types.String   `tfsdk:"project_id"`
	EnvironmentID          types.String   `tfsdk:"environment_id"`
	Name                   types.String   `tfsdk:"name"`
	SourceType             types.String   `tfsdk:"source_type"`
	Repository             types.String   `tfsdk:"repository"`
	Image                  types.String   `tfsdk:"image"`
	Branch                 types.String   `tfsdk:"branch"`
	RootDirectory          types.String   `tfsdk:"root_directory"`
	ConfigPath             types.String   `tfsdk:"config_path"`
	Builder                types.String   `tfsdk:"builder"`
	BuildCommand           types.String   `tfsdk:"build_command"`
	DockerfilePath         types.String   `tfsdk:"dockerfile_path"`
	StartCommand           types.String   `tfsdk:"start_command"`
	PreDeployCommand       types.List     `tfsdk:"pre_deploy_command"`
	HealthcheckPath        types.String   `tfsdk:"healthcheck_path"`
	HealthcheckTimeout     types.Int64    `tfsdk:"healthcheck_timeout"`
	RestartPolicyType      types.String   `tfsdk:"restart_policy_type"`
	RestartPolicyMaxRetry  types.Int64    `tfsdk:"restart_policy_max_retries"`
	Region                 types.String   `tfsdk:"region"`
	ReplicaCount           types.Int64    `tfsdk:"replica_count"`
	Regions                types.Map      `tfsdk:"regions"`
	MemoryGB               types.Float64  `tfsdk:"memory_gb"`
	VCPUs                  types.Float64  `tfsdk:"vcpus"`
	DrainingSeconds        types.Int64    `tfsdk:"draining_seconds"`
	OverlapSeconds         types.Int64    `tfsdk:"overlap_seconds"`
	SleepApplication       types.Bool     `tfsdk:"sleep_application"`
	IPV6EgressEnabled      types.Bool     `tfsdk:"ipv6_egress_enabled"`
	WatchPatterns          types.Set      `tfsdk:"watch_patterns"`
	LatestDeploymentID     types.String   `tfsdk:"latest_deployment_id"`
	LatestDeploymentStatus types.String   `tfsdk:"latest_deployment_status"`
	Timeouts               timeouts.Value `tfsdk:"timeouts"`
}

type serviceEnvironmentConfig struct {
	Services map[string]*struct {
		Deploy *struct {
			MultiRegionConfig map[string]*struct {
				NumReplicas int64 `json:"numReplicas"`
			} `json:"multiRegionConfig"`
		} `json:"deploy"`
	} `json:"services"`
}

func NewService() resource.Resource {
	return &Service{}
}

func (r *Service) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *Service) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	optionalComputedString := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: description}
	}
	optionalComputedInt := func(description string) schema.Int64Attribute {
		return schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: description}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Railway service and its configuration in one environment. Creation is distinct from deployment success; latest deployment status is exposed separately.",
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
			"name": schema.StringAttribute{Required: true},
			"source_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("empty", "github", "image"),
				},
			},
			"repository":     optionalComputedString("GitHub repository in owner/name form."),
			"image":          optionalComputedString("Docker image reference."),
			"branch":         optionalComputedString("Git branch."),
			"root_directory": optionalComputedString("Repository root directory for this service."),
			"config_path":    optionalComputedString("Railway configuration file path."),
			"builder": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf("NIXPACKS", "RAILPACK", "PAKETO", "HEROKU"),
				},
			},
			"build_command":   optionalComputedString("Custom build command."),
			"dockerfile_path": optionalComputedString("Dockerfile path."),
			"start_command":   optionalComputedString("Service start command."),
			"pre_deploy_command": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"healthcheck_path":           optionalComputedString("HTTP health-check path."),
			"healthcheck_timeout":        optionalComputedInt("Health-check timeout in seconds."),
			"restart_policy_type":        optionalComputedString("Restart policy: ALWAYS, ON_FAILURE, or NEVER."),
			"restart_policy_max_retries": optionalComputedInt("Maximum restart attempts for ON_FAILURE."),
			"region":                     optionalComputedString("Single deployment region."),
			"replica_count":              optionalComputedInt("Replica count for a single-region service."),
			"draining_seconds":           optionalComputedInt("Connection draining duration."),
			"overlap_seconds":            optionalComputedInt("Old/new deployment overlap duration."),
			"sleep_application":          schema.BoolAttribute{Optional: true, Computed: true},
			"ipv6_egress_enabled":        schema.BoolAttribute{Optional: true, Computed: true},
			"memory_gb":                  schema.Float64Attribute{Optional: true, Computed: true},
			"vcpus":                      schema.Float64Attribute{Optional: true, Computed: true},
			"latest_deployment_id":       schema.StringAttribute{Computed: true},
			"latest_deployment_status":   schema.StringAttribute{Computed: true},
			"regions": schema.MapAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "Region-to-replica-count map. Entries are normalized by key.",
			},
			"watch_patterns": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"timeouts": timeouts.AttributesAll(ctx),
		},
	}
}

func (r *Service) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *Service) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || !validateServiceSource(&plan, &resp.Diagnostics) {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutCreate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	unlockEnvironment := lockEnvironmentChangeSet(plan.EnvironmentID.ValueString())
	defer unlockEnvironment()
	result, err := railway.CreateService(ctx, r.client.GraphQL(), railway.ServiceCreateInput{
		ProjectId:     plan.ProjectID.ValueString(),
		EnvironmentId: stringPointer(plan.EnvironmentID),
		Name:          stringPointer(plan.Name),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Railway service", client.DecodeAPIError(err).Error())
		return
	}
	plan.ID = types.StringValue(result.ServiceCreate.Id)

	// The service exists in Railway now, so every failure path below must still
	// record it in state.
	//
	// Returning early after a successful create makes Terraform discard the plan
	// and persist nothing. The service is real, Terraform does not know about
	// it, and the next apply fails with `A service named "x" already exists in
	// this project` — a loop with no way out except deleting the service by
	// hand, which is exactly what the previous error message had to ask for.
	//
	// Saving the partial state means a retry updates the service it already
	// created. The apply still fails and still reports why; the failure is
	// simply recoverable rather than terminal.
	//
	// This is the usual convention for a resource whose creation is not one
	// atomic call: persist as soon as the remote object exists, then continue
	// configuring it.
	// **EVERY VALUE MUST BE KNOWN WHEN STATE IS WRITTEN.** Terraform rejects an
	// apply result that still contains an unknown:
	//
	//   Provider returned invalid result object after apply … the provider
	//   still indicated an unknown value for … .builder
	//
	// On the happy path `refresh` resolves those from Railway. On a failure
	// path it has not run, so anything Optional+Computed the plan left unknown
	// is still unknown — and persisting that is a second bug rather than a fix.
	//
	// `ResolveUnknowns` nulls ONLY the unknowns, leaving configured values
	// alone. Nulling everything would discard the practitioner's own
	// configuration from state, which is worse than the orphan being fixed.
	saveState := func() {
		ResolveUnknowns(&plan)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}

	if plan.SourceType.ValueString() != "empty" {
		_, err = railway.ConnectService(ctx, r.client.GraphQL(), plan.ID.ValueString(), railway.ServiceConnectInput{
			Branch: stringPointer(plan.Branch),
			Image:  stringPointer(plan.Image),
			Repo:   stringPointer(plan.Repository),
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Railway service created but source connection failed",
				"The service was created with ID "+plan.ID.ValueString()+
					" and has been saved to state. Its source could not be connected; "+
					"correct the problem and apply again to retry the connection. "+
					client.DecodeAPIError(err).Error(),
			)
			saveState()
			return
		}
	}
	if !r.updateInstance(ctx, &plan, &resp.Diagnostics) {
		saveState()
		return
	}
	if !r.refresh(ctx, &plan, true, &resp.Diagnostics) {
		saveState()
		return
	}
	saveState()
}

func (r *Service) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutRead, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	if !r.refresh(ctx, &state, false, &resp.Diagnostics) {
		if len(resp.Diagnostics.Errors()) == 0 {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Service) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceModel
	var prior serviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() || !validateServiceSource(&plan, &resp.Diagnostics) {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutUpdate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	unlockEnvironment := lockEnvironmentChangeSet(plan.EnvironmentID.ValueString())
	defer unlockEnvironment()
	if plan.Name.ValueString() != prior.Name.ValueString() {
		_, err := railway.UpdateService(ctx, r.client.GraphQL(), plan.ID.ValueString(), railway.ServiceUpdateInput{
			Name: stringPointer(plan.Name),
		})
		if err != nil {
			resp.Diagnostics.AddError("Unable to rename Railway service", client.DecodeAPIError(err).Error())
			return
		}
	}
	if sourceChanged(&plan, &prior) {
		input := railway.ServiceConnectInput{
			Branch: stringPointer(plan.Branch),
			Image:  stringPointer(plan.Image),
			Repo:   stringPointer(plan.Repository),
		}
		if plan.SourceType.ValueString() == "empty" {
			input = railway.ServiceConnectInput{}
		}
		if _, err := railway.ConnectService(ctx, r.client.GraphQL(), plan.ID.ValueString(), input); err != nil {
			resp.Diagnostics.AddError("Unable to update Railway service source", client.DecodeAPIError(err).Error())
			return
		}
	}
	if !r.updateInstance(ctx, &plan, &resp.Diagnostics) {
		return
	}
	if !r.refresh(ctx, &plan, true, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Service) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceModel
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
	environmentID := state.EnvironmentID.ValueString()
	_, err := railway.DeleteService(ctx, r.client.GraphQL(), state.ID.ValueString(), &environmentID)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Railway service", client.DecodeAPIError(err).Error())
	}
}

func (r *Service) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, diagnostics := splitImportID(req.ID, 3)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

func (r *Service) updateInstance(ctx context.Context, plan *serviceModel, diagnostics *diag.Diagnostics) bool {
	var multiRegion *json.RawMessage
	if !plan.Regions.IsNull() && !plan.Regions.IsUnknown() {
		regions := map[string]int64{}
		diagnostics.Append(plan.Regions.ElementsAs(ctx, &regions, false)...)
		config := make(map[string]map[string]int64, len(regions))
		for region, replicas := range regions {
			config[region] = map[string]int64{"numReplicas": replicas}
		}
		encoded, err := json.Marshal(config)
		if err != nil {
			diagnostics.AddError("Unable to encode Railway region configuration", err.Error())
			return false
		}
		raw := json.RawMessage(encoded)
		multiRegion = &raw
	}
	preDeploy := listStrings(ctx, plan.PreDeployCommand, diagnostics)
	watchPatterns := setStrings(ctx, plan.WatchPatterns, diagnostics)
	if diagnostics.HasError() {
		return false
	}
	input := railway.ServiceInstanceUpdateInput{
		BuildCommand:            stringPointer(plan.BuildCommand),
		Builder:                 builderPointer(plan.Builder),
		DockerfilePath:          stringPointer(plan.DockerfilePath),
		DrainingSeconds:         intPointer(plan.DrainingSeconds),
		HealthcheckPath:         stringPointer(plan.HealthcheckPath),
		HealthcheckTimeout:      intPointer(plan.HealthcheckTimeout),
		Ipv6EgressEnabled:       boolPointer(plan.IPV6EgressEnabled),
		MultiRegionConfig:       multiRegion,
		NumReplicas:             intPointer(plan.ReplicaCount),
		OverlapSeconds:          intPointer(plan.OverlapSeconds),
		PreDeployCommand:        preDeploy,
		RailwayConfigFile:       stringPointer(plan.ConfigPath),
		Region:                  stringPointer(plan.Region),
		RestartPolicyMaxRetries: intPointer(plan.RestartPolicyMaxRetry),
		RestartPolicyType:       restartPolicyPointer(plan.RestartPolicyType),
		RootDirectory:           stringPointer(plan.RootDirectory),
		SleepApplication:        boolPointer(plan.SleepApplication),
		StartCommand:            stringPointer(plan.StartCommand),
		WatchPatterns:           watchPatterns,
	}
	_, err := railway.UpdateServiceInstance(
		ctx,
		r.client.GraphQL(),
		plan.EnvironmentID.ValueString(),
		plan.ID.ValueString(),
		input,
	)
	if err != nil {
		diagnostics.AddError("Unable to update Railway service instance", client.DecodeAPIError(err).Error())
		return false
	}

	memoryConfigured := !plan.MemoryGB.IsNull() && !plan.MemoryGB.IsUnknown()
	vcpusConfigured := !plan.VCPUs.IsNull() && !plan.VCPUs.IsUnknown()
	if memoryConfigured || vcpusConfigured {
		_, err = railway.UpdateServiceInstanceLimits(ctx, r.client.GraphQL(), railway.ServiceInstanceLimitsUpdateInput{
			EnvironmentId: plan.EnvironmentID.ValueString(),
			ServiceId:     plan.ID.ValueString(),
			MemoryGB:      floatPointer(plan.MemoryGB),
			VCPUs:         floatPointer(plan.VCPUs),
		})
		if err != nil {
			diagnostics.AddError("Unable to update Railway service resource limits", client.DecodeAPIError(err).Error())
			return false
		}
	}
	return true
}

func (r *Service) refresh(
	ctx context.Context,
	state *serviceModel,
	preserveConfiguredSource bool,
	diagnostics *diag.Diagnostics,
) bool {
	configuredSourceType := state.SourceType
	configuredRepository := state.Repository
	configuredImage := state.Image
	configuredBranch := state.Branch
	result, err := railway.GetService(
		ctx,
		r.client.GraphQL(),
		state.ID.ValueString(),
		state.EnvironmentID.ValueString(),
	)
	if client.IsNotFound(err) {
		return false
	}
	if err != nil {
		diagnostics.AddError("Unable to read Railway service", client.DecodeAPIError(err).Error())
		return false
	}
	if result.Service.DeletedAt != nil {
		return false
	}
	state.ID = types.StringValue(result.Service.Id)
	state.ProjectID = types.StringValue(result.Service.ProjectId)
	state.Name = types.StringValue(result.Service.Name)
	resetServiceOptionalComputedState(state)

	var instance *railway.ServiceInstanceFields
	for _, edge := range result.Environment.ServiceInstances.Edges {
		if edge.Node.ServiceId == state.ID.ValueString() {
			copy := edge.Node.ServiceInstanceFields
			instance = &copy
			break
		}
	}
	if instance == nil {
		return false
	}
	setServiceInstanceState(ctx, state, instance, diagnostics)

	branchFound := false
	for _, edge := range result.Service.RepoTriggers.Edges {
		if edge.Node.EnvironmentId == state.EnvironmentID.ValueString() {
			state.Branch = types.StringValue(edge.Node.Branch)
			branchFound = true
			break
		}
	}
	// Railway can expose a newly-created service instance before its connected
	// source and repo trigger converge. During Create/Update only, retain the
	// already-known configured values to satisfy Terraform's post-apply
	// consistency contract. Ordinary Read never preserves them and therefore
	// remains authoritative for drift detection.
	if preserveConfiguredSource && state.SourceType.ValueString() == "empty" &&
		(configuredSourceType.ValueString() == "github" || configuredSourceType.ValueString() == "image") {
		state.SourceType = configuredSourceType
		if configuredSourceType.ValueString() == "github" {
			state.Repository = configuredRepository
			state.Image = types.StringNull()
		} else {
			state.Repository = types.StringNull()
			state.Image = configuredImage
		}
	}
	if preserveConfiguredSource && !branchFound && !configuredBranch.IsUnknown() {
		state.Branch = configuredBranch
	}
	var opaque serviceEnvironmentConfig
	if json.Unmarshal(result.Environment.Config, &opaque) == nil {
		if config := opaque.Services[state.ID.ValueString()]; config != nil && config.Deploy != nil &&
			config.Deploy.MultiRegionConfig != nil {
			regions := make(map[string]int64, len(config.Deploy.MultiRegionConfig))
			for region, value := range config.Deploy.MultiRegionConfig {
				if value != nil {
					regions[region] = value.NumReplicas
				}
			}
			state.Regions = mapIntToTerraform(ctx, regions, diagnostics)
		}
	}
	if result.LimitOverride != nil && len(*result.LimitOverride) > 0 {
		var limits struct {
			MemoryGB *float64 `json:"memoryGB"`
			VCPUs    *float64 `json:"vCPUs"`
		}
		if json.Unmarshal(*result.LimitOverride, &limits) == nil {
			if limits.MemoryGB != nil {
				state.MemoryGB = types.Float64Value(*limits.MemoryGB)
			}
			if limits.VCPUs != nil {
				state.VCPUs = types.Float64Value(*limits.VCPUs)
			}
		}
	}
	return !diagnostics.HasError()
}

func resetServiceOptionalComputedState(state *serviceModel) {
	state.Repository = types.StringNull()
	state.Image = types.StringNull()
	state.Branch = types.StringNull()
	state.RootDirectory = types.StringNull()
	state.ConfigPath = types.StringNull()
	state.Builder = types.StringNull()
	state.BuildCommand = types.StringNull()
	state.DockerfilePath = types.StringNull()
	state.StartCommand = types.StringNull()
	state.PreDeployCommand = types.ListNull(types.StringType)
	state.HealthcheckPath = types.StringNull()
	state.HealthcheckTimeout = types.Int64Null()
	state.RestartPolicyType = types.StringNull()
	state.RestartPolicyMaxRetry = types.Int64Null()
	state.Region = types.StringNull()
	state.ReplicaCount = types.Int64Null()
	state.Regions = types.MapNull(types.Int64Type)
	state.MemoryGB = types.Float64Null()
	state.VCPUs = types.Float64Null()
	state.DrainingSeconds = types.Int64Null()
	state.OverlapSeconds = types.Int64Null()
	state.SleepApplication = types.BoolNull()
	state.IPV6EgressEnabled = types.BoolNull()
	state.WatchPatterns = types.SetNull(types.StringType)
}

func setServiceInstanceState(ctx context.Context, state *serviceModel, remote *railway.ServiceInstanceFields, diagnostics *diag.Diagnostics) {
	state.BuildCommand = valueString(remote.BuildCommand)
	state.Builder = types.StringValue(string(remote.Builder))
	state.DockerfilePath = valueString(remote.DockerfilePath)
	state.DrainingSeconds = intValue(remote.DrainingSeconds)
	state.HealthcheckPath = valueString(remote.HealthcheckPath)
	state.HealthcheckTimeout = intValue(remote.HealthcheckTimeout)
	state.IPV6EgressEnabled = boolValue(remote.Ipv6EgressEnabled)
	state.ReplicaCount = intValue(remote.NumReplicas)
	state.OverlapSeconds = intValue(remote.OverlapSeconds)
	state.ConfigPath = valueString(remote.RailwayConfigFile)
	state.Region = valueString(remote.Region)
	state.RestartPolicyMaxRetry = types.Int64Value(int64(remote.RestartPolicyMaxRetries))
	state.RestartPolicyType = types.StringValue(string(remote.RestartPolicyType))
	state.RootDirectory = valueString(remote.RootDirectory)
	state.SleepApplication = boolValue(remote.SleepApplication)
	state.StartCommand = valueString(remote.StartCommand)
	state.WatchPatterns = setToTerraform(ctx, remote.WatchPatterns, diagnostics)
	if remote.PreDeployCommand != nil {
		var commands []string
		if json.Unmarshal(*remote.PreDeployCommand, &commands) == nil {
			state.PreDeployCommand = listToTerraform(ctx, commands, diagnostics)
		}
	}
	if remote.Source != nil {
		if remote.Source.Repo != nil {
			state.SourceType = types.StringValue("github")
			state.Repository = valueString(remote.Source.Repo)
			state.Image = types.StringNull()
		} else if remote.Source.Image != nil {
			state.SourceType = types.StringValue("image")
			state.Image = valueString(remote.Source.Image)
			state.Repository = types.StringNull()
		} else {
			state.SourceType = types.StringValue("empty")
			state.Repository = types.StringNull()
			state.Image = types.StringNull()
		}
	} else {
		state.SourceType = types.StringValue("empty")
		state.Repository = types.StringNull()
		state.Image = types.StringNull()
	}
	if remote.LatestDeployment != nil {
		state.LatestDeploymentID = types.StringValue(remote.LatestDeployment.Id)
		state.LatestDeploymentStatus = types.StringValue(string(remote.LatestDeployment.Status))
	} else {
		state.LatestDeploymentID = types.StringNull()
		state.LatestDeploymentStatus = types.StringNull()
	}
}

func validateServiceSource(plan *serviceModel, diagnostics *diag.Diagnostics) bool {
	switch plan.SourceType.ValueString() {
	case "empty":
		if (!plan.Repository.IsNull() && !plan.Repository.IsUnknown()) ||
			(!plan.Image.IsNull() && !plan.Image.IsUnknown()) {
			diagnostics.AddError("Invalid Railway service source", "source_type empty cannot be combined with repository or image.")
			return false
		}
	case "github":
		if plan.Repository.IsNull() || plan.Repository.IsUnknown() || plan.Repository.ValueString() == "" {
			diagnostics.AddError("Invalid Railway service source", "source_type github requires repository.")
			return false
		}
	case "image":
		if plan.Image.IsNull() || plan.Image.IsUnknown() || plan.Image.ValueString() == "" {
			diagnostics.AddError("Invalid Railway service source", "source_type image requires image.")
			return false
		}
	}
	return true
}

func sourceChanged(plan, prior *serviceModel) bool {
	return plan.SourceType.ValueString() != prior.SourceType.ValueString() ||
		plan.Repository.ValueString() != prior.Repository.ValueString() ||
		plan.Image.ValueString() != prior.Image.ValueString() ||
		plan.Branch.ValueString() != prior.Branch.ValueString()
}

func builderPointer(value types.String) *railway.Builder {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := railway.Builder(value.ValueString())
	return &result
}

func restartPolicyPointer(value types.String) *railway.RestartPolicyType {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := railway.RestartPolicyType(value.ValueString())
	return &result
}

func floatPointer(value types.Float64) *float64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := value.ValueFloat64()
	return &result
}

func intValue(value *int) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func boolValue(value *bool) types.Bool {
	if value == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*value)
}

func listStrings(ctx context.Context, value types.List, diagnostics *diag.Diagnostics) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var result []string
	diagnostics.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

func setStrings(ctx context.Context, value types.Set, diagnostics *diag.Diagnostics) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var result []string
	diagnostics.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

func listToTerraform(ctx context.Context, values []string, diagnostics *diag.Diagnostics) types.List {
	result, converted := types.ListValueFrom(ctx, types.StringType, values)
	diagnostics.Append(converted...)
	return result
}

func setToTerraform(ctx context.Context, values []string, diagnostics *diag.Diagnostics) types.Set {
	result, converted := types.SetValueFrom(ctx, types.StringType, values)
	diagnostics.Append(converted...)
	return result
}

func mapIntToTerraform(ctx context.Context, values map[string]int64, diagnostics *diag.Diagnostics) types.Map {
	result, converted := types.MapValueFrom(ctx, types.Int64Type, values)
	diagnostics.Append(converted...)
	return result
}

var _ = fmt.Sprintf
