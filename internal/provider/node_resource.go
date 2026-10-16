package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

var (
	_ resource.Resource                     = (*nodeResource)(nil)
	_ resource.ResourceWithConfigure        = (*nodeResource)(nil)
	_ resource.ResourceWithImportState      = (*nodeResource)(nil)
	_ resource.ResourceWithConfigValidators = (*nodeResource)(nil)
	_ resource.ResourceWithModifyPlan       = (*nodeResource)(nil)
)

// defaultComputeID is sent when compute_id is unset: built-in templates have
// compute_id null and POST /v2/projects/{project_id}/nodes requires one.
const defaultComputeID = "local"

// consoleTypes is the console_type enum from
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
var consoleTypes = []string{"vnc", "telnet", "http", "https", "spice", "spice+agent", "none"}

// nodeTypes is the node_type enum from the same page.
var nodeTypes = []string{
	"cloud", "nat", "ethernet_hub", "ethernet_switch", "frame_relay_switch", "atm_switch",
	"docker", "dynamips", "vpcs", "traceng", "virtualbox", "vmware", "iou", "qemu",
}

// NewNodeResource is the gns3_node resource.
func NewNodeResource() resource.Resource { return &nodeResource{} }

type nodeResource struct{ client gns3.Client }

type nodeResourceModel struct {
	ProjectID     types.String         `tfsdk:"project_id"`
	TemplateID    types.String         `tfsdk:"template_id"`
	NodeType      types.String         `tfsdk:"node_type"`
	ComputeID     types.String         `tfsdk:"compute_id"`
	Properties    jsontypes.Normalized `tfsdk:"properties"`
	Name          types.String         `tfsdk:"name"`
	X             types.Int64          `tfsdk:"x"`
	Y             types.Int64          `tfsdk:"y"`
	Z             types.Int64          `tfsdk:"z"`
	Symbol        types.String         `tfsdk:"symbol"`
	ConsoleType   types.String         `tfsdk:"console_type"`
	StartOnCreate types.Bool           `tfsdk:"start_on_create"`
	NodeID        types.String         `tfsdk:"node_id"`
	Console       types.Int64          `tfsdk:"console"`
	Status        types.String         `tfsdk:"status"`
	Ports         types.List           `tfsdk:"ports"`
}

var portAttrTypes = map[string]attr.Type{
	"adapter_number": types.Int64Type,
	"port_number":    types.Int64Type,
	"name":           types.StringType,
	"link_type":      types.StringType,
}

func (r *nodeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

func (r *nodeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A node in a GNS3 project, created from a template (`template_id`) or directly " +
			"from an emulator type (`node_type` with `compute_id` and `properties`). Exactly one of `template_id` " +
			"and `node_type` must be set. The node's ports are read back after create so `gns3_link` can reference them.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project UUID. Changing it replaces the node.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"template_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Template UUID to create the node from, for example from the `gns3_template` data source. Changing it replaces the node.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"node_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Emulator type for a template-less node: one of `" + joinTicks(nodeTypes) + "`. " +
					"Read from the server when `template_id` is used. Changing it replaces the node.",
				Validators: []validator.String{stringvalidator.OneOf(nodeTypes...)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"compute_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Compute to run the node on. When unset, `local` is sent; a template with " +
					"its own compute keeps it. Changing it replaces the node.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"properties": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
				MarkdownDescription: "Emulator-specific properties as a JSON object (use `jsonencode`), only with `node_type`. " +
					"Only the keys you set are compared with the server, so server-side defaults do not show as drift. " +
					"For `vpcs`, `startup_script` is written to and read from the node's `startup.vpc` file rather than sent as a property, " +
					"because GNS3 2.2 rejects it in node updates.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Node name, unique within the project.",
			},
			"x": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				MarkdownDescription: "X position on the drawing area. Default `0`.",
			},
			"y": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				MarkdownDescription: "Y position on the drawing area. Default `0`.",
			},
			"z": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(1),
				MarkdownDescription: "Z order on the drawing area. Default `1`.",
			},
			"symbol": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Symbol path, for example `:/symbols/computer.svg`. The template or node type default when unset.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"console_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Console type: one of `" + joinTicks(consoleTypes) + "`. The template or node type default when unset.",
				Validators:          []validator.String{stringvalidator.OneOf(consoleTypes...)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"start_on_create": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Start the node right after it is created. Only acts at create; later changes are recorded but do not start or stop the node. Default `false`.",
			},
			"node_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Node UUID assigned by the server.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"console": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Console TCP port, null when the node has no console.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`stopped`, `started` or `suspended`, as last read from the server.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ports": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The node's ports as reported by the server.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"adapter_number": schema.Int64Attribute{Computed: true, MarkdownDescription: "Adapter number."},
						"port_number":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Port number on the adapter."},
						"name":           schema.StringAttribute{Computed: true, MarkdownDescription: "Port name, for example `Ethernet0`."},
						"link_type":      schema.StringAttribute{Computed: true, MarkdownDescription: "`ethernet` or `serial`."},
					},
				},
			},
		},
	}
}

func (r *nodeResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("template_id"), path.MatchRoot("node_type")),
		resourcevalidator.Conflicting(path.MatchRoot("template_id"), path.MatchRoot("properties")),
	}
}

// ModifyPlan marks ports and console unknown when an in-place change can
// alter them, so UseStateForUnknown does not promise stale values.
func (r *nodeResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state nodeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	propsChanged := !plan.Properties.IsNull() && !plan.Properties.Equal(state.Properties)
	consoleChanged := !plan.ConsoleType.Equal(state.ConsoleType)
	if propsChanged {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("ports"), types.ListUnknown(types.ObjectType{AttrTypes: portAttrTypes}))...)
	}
	if propsChanged || consoleChanged {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("console"), types.Int64Unknown())...)
	}
}

func (r *nodeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

// fromAPI copies server values into m. properties is not touched; see
// readProperties.
func (m *nodeResourceModel) fromAPI(n *gns3.Node, diags *diag.Diagnostics) {
	m.ProjectID = types.StringValue(n.ProjectID)
	m.NodeID = types.StringValue(n.NodeID)
	m.TemplateID = stringOrNull(n.TemplateID)
	m.NodeType = types.StringValue(n.NodeType)
	m.ComputeID = types.StringValue(n.ComputeID)
	m.Name = types.StringValue(n.Name)
	m.X = types.Int64Value(n.X)
	m.Y = types.Int64Value(n.Y)
	m.Z = types.Int64Value(n.Z)
	m.Symbol = stringOrNull(n.Symbol)
	m.ConsoleType = stringOrNull(n.ConsoleType)
	m.Status = types.StringValue(n.Status)
	if n.Console == nil {
		m.Console = types.Int64Null()
	} else {
		m.Console = types.Int64Value(*n.Console)
	}
	ports := make([]attr.Value, 0, len(n.Ports))
	for _, p := range n.Ports {
		obj, d := types.ObjectValue(portAttrTypes, map[string]attr.Value{
			"adapter_number": types.Int64Value(p.AdapterNumber),
			"port_number":    types.Int64Value(p.PortNumber),
			"name":           types.StringValue(p.Name),
			"link_type":      types.StringValue(p.LinkType),
		})
		diags.Append(d...)
		ports = append(ports, obj)
	}
	list, d := types.ListValue(types.ObjectType{AttrTypes: portAttrTypes}, ports)
	diags.Append(d...)
	m.Ports = list
}

// decodeProperties parses the properties attribute into raw JSON per key.
func decodeProperties(v jsontypes.Normalized, diags *diag.Diagnostics) map[string]json.RawMessage {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal([]byte(v.ValueString()), &props); err != nil {
		diags.AddAttributeError(path.Root("properties"), "Invalid properties", "properties must be a JSON object: "+err.Error())
		return nil
	}
	return props
}

// fileProperties lists, per node type, the properties GNS3 2.2 keeps as a file
// in the node directory. POST .../nodes accepts them, but the controller then
// caches them and forwards them on every later PUT to a compute schema that
// rejects them (HTTP 400 "Additional properties are not allowed", seen on
// 2.2.61, docs/api-drift.md), and the cached value goes stale once the file
// changes. The provider writes and reads them through the node files endpoint
// and never sends them in properties.
var fileProperties = map[string]map[string]string{
	"vpcs": {"startup_script": "startup.vpc"},
}

// splitProperties returns the properties to send to the API and the
// file-backed ones as file path -> content.
func splitProperties(nodeType string, props map[string]json.RawMessage, diags *diag.Diagnostics) (map[string]json.RawMessage, map[string]string) {
	var rest map[string]json.RawMessage
	files := map[string]string{}
	for k, v := range props {
		file, ok := fileProperties[nodeType][k]
		if !ok {
			if rest == nil {
				rest = map[string]json.RawMessage{}
			}
			rest[k] = v
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			diags.AddAttributeError(path.Root("properties"), "Invalid properties", fmt.Sprintf("%s must be a string: %v", k, err))
			continue
		}
		files[file] = s
	}
	return rest, files
}

// writeNodeFiles writes file-backed properties in path order.
func writeNodeFiles(ctx context.Context, c gns3.Client, projectID, nodeID string, files map[string]string) error {
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if err := c.WriteNodeFile(ctx, projectID, nodeID, p, []byte(files[p])); err != nil {
			return err
		}
	}
	return nil
}

// serverProperties is n.Properties with each file-backed key that state
// manages replaced by the file's content (dropped when the file is missing).
func serverProperties(ctx context.Context, c gns3.Client, n *gns3.Node, state jsontypes.Normalized, diags *diag.Diagnostics) map[string]json.RawMessage {
	keys := decodeProperties(state, diags)
	out := maps.Clone(n.Properties)
	for k := range keys {
		file, ok := fileProperties[n.NodeType][k]
		if !ok {
			continue
		}
		if out == nil {
			out = map[string]json.RawMessage{}
		}
		content, err := c.ReadNodeFile(ctx, n.ProjectID, n.NodeID, file)
		if gns3.IsNotFound(err) {
			delete(out, k)
			continue
		}
		if err != nil {
			diags.AddError("Error reading GNS3 node file", err.Error())
			return n.Properties
		}
		buf, _ := json.Marshal(string(content))
		out[k] = buf
	}
	return out
}

// readProperties keeps only the keys already in state, with the server's
// values, so emulator defaults the user never set do not show as drift.
func readProperties(state jsontypes.Normalized, server map[string]json.RawMessage, diags *diag.Diagnostics) jsontypes.Normalized {
	keys := decodeProperties(state, diags)
	if keys == nil {
		return state
	}
	out := make(map[string]json.RawMessage, len(keys))
	for k := range keys {
		if v, ok := server[k]; ok {
			out[k] = v
		}
	}
	buf, err := json.Marshal(out)
	if err != nil {
		diags.AddError("Error encoding node properties", err.Error())
		return state
	}
	return jsontypes.NewNormalizedValue(string(buf))
}

func (r *nodeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nodeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID := plan.ProjectID.ValueString()
	computeID := defaultComputeID
	if c := knownString(plan.ComputeID); c != nil {
		computeID = *c
	}

	var n *gns3.Node
	var err error
	var files map[string]string
	if !plan.TemplateID.IsNull() {
		n, err = r.client.CreateNodeFromTemplate(ctx, projectID, plan.TemplateID.ValueString(), gns3.NodeFromTemplateRequest{
			X: plan.X.ValueInt64(), Y: plan.Y.ValueInt64(), Name: knownString(plan.Name), ComputeID: &computeID,
		})
	} else {
		var props map[string]json.RawMessage
		props, files = splitProperties(plan.NodeType.ValueString(), decodeProperties(plan.Properties, &resp.Diagnostics), &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		n, err = r.client.CreateNode(ctx, projectID, gns3.NodeCreateRequest{
			ComputeID: computeID, Name: plan.Name.ValueString(), NodeType: plan.NodeType.ValueString(),
			ConsoleType: knownString(plan.ConsoleType), Symbol: knownString(plan.Symbol),
			X: knownInt64(plan.X), Y: knownInt64(plan.Y), Z: knownInt64(plan.Z), Properties: props,
		})
	}
	if err != nil {
		resp.Diagnostics.AddError("Error creating GNS3 node", err.Error())
		return
	}
	nodeID := n.NodeID
	// From here on the node exists; record it before any further call can fail.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("node_id"), nodeID)...)

	if err := writeNodeFiles(ctx, r.client, projectID, nodeID, files); err != nil {
		resp.Diagnostics.AddError("Error writing GNS3 node file after create", err.Error())
		return
	}

	// The template endpoint takes only x, y, name and compute_id; apply the
	// rest with a PUT when the configuration asks for something different.
	if !plan.TemplateID.IsNull() && needsTemplateFollowUp(plan, n) {
		if _, err := r.client.UpdateNode(ctx, projectID, nodeID, gns3.NodeUpdateRequest{
			ComputeID: n.ComputeID, Name: n.Name, NodeType: n.NodeType,
			Z: knownInt64(plan.Z), Symbol: knownString(plan.Symbol), ConsoleType: knownString(plan.ConsoleType),
		}); err != nil {
			resp.Diagnostics.AddError("Error updating GNS3 node after create from template", err.Error())
			return
		}
	}
	if plan.StartOnCreate.ValueBool() {
		if _, err := r.client.StartNode(ctx, projectID, nodeID); err != nil {
			resp.Diagnostics.AddError("Error starting GNS3 node (start_on_create)", err.Error())
			return
		}
	}

	// Read back so ports, console and status are what the server now reports.
	n, err = r.client.GetNode(ctx, projectID, nodeID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading GNS3 node after create", err.Error())
		return
	}
	plan.fromAPI(n, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func needsTemplateFollowUp(plan nodeResourceModel, n *gns3.Node) bool {
	if z := knownInt64(plan.Z); z != nil && *z != n.Z {
		return true
	}
	if s := knownString(plan.Symbol); s != nil && (n.Symbol == nil || *s != *n.Symbol) {
		return true
	}
	if c := knownString(plan.ConsoleType); c != nil && (n.ConsoleType == nil || *c != *n.ConsoleType) {
		return true
	}
	return false
}

func (r *nodeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nodeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID, nodeID := state.ProjectID.ValueString(), state.NodeID.ValueString()
	n, err := r.client.GetNode(ctx, projectID, nodeID)
	if gns3.IsNotFound(err) {
		if explainMissing(ctx, r.client, projectID, "node "+nodeID, &resp.Diagnostics) {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading GNS3 node", err.Error())
		return
	}
	state.fromAPI(n, &resp.Diagnostics)
	state.Properties = readProperties(state.Properties, serverProperties(ctx, r.client, n, state.Properties, &resp.Diagnostics), &resp.Diagnostics)
	if state.StartOnCreate.IsNull() || state.StartOnCreate.IsUnknown() {
		state.StartOnCreate = types.BoolValue(false) // after import
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *nodeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state nodeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID, nodeID := state.ProjectID.ValueString(), state.NodeID.ValueString()

	changed := !plan.Name.Equal(state.Name) || !plan.X.Equal(state.X) || !plan.Y.Equal(state.Y) ||
		!plan.Z.Equal(state.Z) || !plan.Symbol.Equal(state.Symbol) || !plan.ConsoleType.Equal(state.ConsoleType) ||
		(!plan.Properties.IsNull() && !plan.Properties.Equal(state.Properties))
	if changed {
		props, files := splitProperties(state.NodeType.ValueString(), decodeProperties(plan.Properties, &resp.Diagnostics), &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		if _, err := r.client.UpdateNode(ctx, projectID, nodeID, gns3.NodeUpdateRequest{
			ComputeID: state.ComputeID.ValueString(), Name: plan.Name.ValueString(), NodeType: state.NodeType.ValueString(),
			X: knownInt64(plan.X), Y: knownInt64(plan.Y), Z: knownInt64(plan.Z),
			Symbol: knownString(plan.Symbol), ConsoleType: knownString(plan.ConsoleType), Properties: props,
		}); err != nil {
			resp.Diagnostics.AddError("Error updating GNS3 node", err.Error())
			return
		}
		if err := writeNodeFiles(ctx, r.client, projectID, nodeID, files); err != nil {
			resp.Diagnostics.AddError("Error writing GNS3 node file", err.Error())
			return
		}
	}
	n, err := r.client.GetNode(ctx, projectID, nodeID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading GNS3 node after update", err.Error())
		return
	}
	plan.fromAPI(n, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nodeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nodeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteNode(ctx, state.ProjectID.ValueString(), state.NodeID.ValueString())
	if err != nil && !gns3.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting GNS3 node", err.Error())
	}
}

func (r *nodeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, nodeID, ok := splitImportID(req.ID, "<project_id>/<node_id>", &resp.Diagnostics)
	if !ok {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("node_id"), nodeID)...)
}

func joinTicks(vals []string) string { return strings.Join(vals, "`, `") }
