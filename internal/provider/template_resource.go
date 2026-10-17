package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

var (
	_ resource.Resource                   = (*templateResource)(nil)
	_ resource.ResourceWithConfigure      = (*templateResource)(nil)
	_ resource.ResourceWithImportState    = (*templateResource)(nil)
	_ resource.ResourceWithModifyPlan     = (*templateResource)(nil)
	_ resource.ResourceWithValidateConfig = (*templateResource)(nil)
)

// templateTypes is the template_type enum a 2.2.61 POST /v2/templates
// accepts (its 400 for an unknown type lists them; docs/api-drift.md).
var templateTypes = []string{
	"cloud", "nat", "ethernet_hub", "ethernet_switch", "docker", "dynamips", "vpcs", "traceng",
	"virtualbox", "vmware", "iou", "qemu",
}

// requiredTemplateProperties are the type-specific keys a 2.2.61 POST
// /v2/templates requires (400 "'platform' is a required property", seen on
// 2.2.61, docs/api-drift.md). vpcs and qemu need none; the other types were
// not probed.
var requiredTemplateProperties = map[string][]string{
	"dynamips": {"platform", "image"},
	"docker":   {"image"},
}

// NewTemplateResource is the gns3_template resource.
func NewTemplateResource() resource.Resource { return &templateResource{} }

type templateResource struct{ client gns3.Client }

type templateResourceModel struct {
	Name              types.String         `tfsdk:"name"`
	TemplateType      types.String         `tfsdk:"template_type"`
	ComputeID         types.String         `tfsdk:"compute_id"`
	Properties        jsontypes.Normalized `tfsdk:"properties"`
	TemplateID        types.String         `tfsdk:"template_id"`
	Builtin           types.Bool           `tfsdk:"builtin"`
	Category          types.String         `tfsdk:"category"`
	Symbol            types.String         `tfsdk:"symbol"`
	DefaultNameFormat types.String         `tfsdk:"default_name_format"`
}

func (r *templateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_template"
}

func (r *templateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A user template on the GNS3 controller, for `gns3_node.template_id`. " +
			"Built-in templates (VPCS, Ethernet switch, ...) can't be managed or imported; read them with the `gns3_template` data source. " +
			"Destroying the template does not affect nodes already created from it.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Template name. GNS3 allows two templates with the same name. Changed in place.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"template_type": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Emulator type: one of `" + joinTicks(templateTypes) + "`. Changing it replaces the template " +
					"(GNS3 2.2 accepts a new type in an update but ignores it).",
				Validators:    []validator.String{stringvalidator.OneOf(templateTypes...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"compute_id": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(defaultComputeID),
				MarkdownDescription: "Compute that nodes from this template run on. Default `local`. Changing it replaces the template " +
					"(GNS3 2.2 accepts a new compute in an update but ignores it).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"properties": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
				MarkdownDescription: "Type-specific template fields as a JSON object (use `jsonencode`), for example " +
					"`{ ram = 512, adapters = 2, hda_disk_image = \"vyos.qcow2\" }` for `qemu`. " +
					"Only the keys you set are compared with the server, so the defaults GNS3 fills in do not show as drift. " +
					"`dynamips` needs `platform` and `image`, and `docker` needs `image`. " +
					"The common fields (`" + joinTicks(gns3.TemplateCommonKeys) + "`) are refused here. " +
					"A change is applied in place. GNS3 2.2 does not check type-specific keys in an update, so the provider first " +
					"creates a throwaway copy named `tf-validate-<random>` with the new properties, which GNS3 does check, and deletes it again. " +
					"A key you remove from the configuration keeps its value on the server, because a GNS3 update merges keys.",
			},
			"template_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Template UUID assigned by the server, for `gns3_node.template_id`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"builtin": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Always `false`: built-in templates are refused.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"category": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`router`, `switch`, `guest` or `firewall`, as filled in by the server for the template type.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"symbol": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Symbol path, as filled in by the server for the template type.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"default_name_format": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Node name pattern, for example `PC{0}`, as filled in by the server for the template type.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// ValidateConfig checks properties against template_type when both are known.
func (r *templateResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg templateResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.Properties.IsUnknown() {
		return
	}
	props := decodeProperties(cfg.Properties, &resp.Diagnostics)
	for _, k := range slices.Sorted(maps.Keys(props)) {
		if slices.Contains(gns3.TemplateCommonKeys, k) {
			resp.Diagnostics.AddAttributeError(path.Root("properties"), "Common template field in properties",
				fmt.Sprintf("properties.%s is a common template field, not a type-specific one. "+
					"Set name, template_type and compute_id as attributes; the others are filled in by the server.", k))
		}
	}
	if cfg.TemplateType.IsNull() || cfg.TemplateType.IsUnknown() {
		return
	}
	tplType := cfg.TemplateType.ValueString()
	for _, k := range requiredTemplateProperties[tplType] {
		if _, ok := props[k]; !ok {
			resp.Diagnostics.AddAttributeError(path.Root("properties"), "Missing template property",
				fmt.Sprintf("A %s template needs properties.%s: GNS3 2.2 refuses to create one without it.", tplType, k))
		}
	}
}

// ModifyPlan refuses to manage a built-in template.
func (r *templateResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() {
		return
	}
	var state templateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() && state.Builtin.ValueBool() {
		resp.Diagnostics.Append(builtinTemplateError(state.TemplateID.ValueString(), state.Name.ValueString()))
	}
}

func builtinTemplateError(id, name string) diag.Diagnostic {
	return diag.NewErrorDiagnostic("Built-in GNS3 template",
		fmt.Sprintf("Template %q (%s) ships with GNS3 and cannot be managed by gns3_template: GNS3 refuses to delete it (HTTP 409). "+
			"Read it with the gns3_template data source instead, and remove it from state with terraform state rm if it was imported.", name, id))
}

func (r *templateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

// fromAPI copies server values into m. properties is not touched; see
// readProperties.
func (m *templateResourceModel) fromAPI(t *gns3.Template) {
	m.TemplateID = types.StringValue(t.TemplateID)
	m.Name = types.StringValue(t.Name)
	m.TemplateType = types.StringValue(t.TemplateType)
	m.ComputeID = stringOrNull(t.ComputeID)
	m.Builtin = types.BoolValue(t.Builtin)
	m.Category = types.StringValue(t.Category)
	m.Symbol = types.StringValue(t.Symbol)
	m.DefaultNameFormat = types.StringValue(t.DefaultNameFormat)
}

func (r *templateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan templateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	props := decodeProperties(plan.Properties, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.client.CreateTemplate(ctx, gns3.TemplateCreateRequest{
		Name: plan.Name.ValueString(), TemplateType: plan.TemplateType.ValueString(),
		ComputeID: plan.ComputeID.ValueString(), Properties: props,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating GNS3 template", err.Error())
		return
	}
	plan.fromAPI(t)
	plan.Properties = readProperties(plan.Properties, t.Properties, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *templateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state templateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.client.GetTemplate(ctx, state.TemplateID.ValueString())
	if gns3.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading GNS3 template", err.Error())
		return
	}
	if t.Builtin {
		resp.Diagnostics.Append(builtinTemplateError(t.TemplateID, t.Name))
		return
	}
	state.fromAPI(t)
	state.Properties = readProperties(state.Properties, t.Properties, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *templateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state templateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.TemplateID.ValueString()
	props := decodeProperties(plan.Properties, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Properties.IsNull() && !plan.Properties.Equal(state.Properties) {
		rejected, leftover := checkTemplateProperties(ctx, r.client, state.TemplateType.ValueString(), state.ComputeID.ValueString(), props)
		if rejected != nil {
			resp.Diagnostics.AddAttributeError(path.Root("properties"), "GNS3 rejected the template properties",
				fmt.Sprintf("Template %s was not changed. GNS3 2.2 does not check type-specific keys in an update, so the provider "+
					"created a throwaway copy with the new properties to check them, and GNS3 refused it:\n\n%s", id, rejected.Error()))
			return
		}
		if leftover != nil {
			resp.Diagnostics.AddError("Throwaway GNS3 template left behind", fmt.Sprintf("Template %s was not changed: %s", id, leftover.Error()))
			return
		}
	}
	t, err := r.client.UpdateTemplate(ctx, id, gns3.TemplateUpdateRequest{Name: plan.Name.ValueString(), Properties: props})
	if err != nil {
		resp.Diagnostics.AddError("Error updating GNS3 template", err.Error())
		return
	}
	plan.fromAPI(t)
	plan.Properties = readProperties(plan.Properties, t.Properties, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// checkTemplateProperties creates and deletes a throwaway template with
// props, because 2.2.61 checks type-specific keys on POST /v2/templates but
// stores anything a PUT sends (docs/api-drift.md). rejected is the failed
// create; leftover names a copy that was created but could not be deleted.
func checkTemplateProperties(ctx context.Context, c gns3.Client, tplType, computeID string, props map[string]json.RawMessage) (rejected, leftover error) {
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	t, err := c.CreateTemplate(ctx, gns3.TemplateCreateRequest{
		Name: "tf-validate-" + hex.EncodeToString(suffix), TemplateType: tplType, ComputeID: computeID, Properties: props,
	})
	if err != nil {
		return err, nil
	}
	if err := c.DeleteTemplate(ctx, t.TemplateID); err != nil && !gns3.IsNotFound(err) {
		return nil, fmt.Errorf("the new properties are valid, but the throwaway template %q (%s) that checked them could not be deleted; "+
			"delete it by hand (DELETE /v2/templates/%s):\n\n%s", t.Name, t.TemplateID, t.TemplateID, err.Error())
	}
	return nil, nil
}

func (r *templateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state templateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteTemplate(ctx, state.TemplateID.ValueString())
	if err != nil && !gns3.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting GNS3 template", err.Error())
	}
}

func (r *templateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("template_id"), req, resp)
}
