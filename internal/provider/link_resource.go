package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

var (
	_ resource.Resource                = (*linkResource)(nil)
	_ resource.ResourceWithConfigure   = (*linkResource)(nil)
	_ resource.ResourceWithImportState = (*linkResource)(nil)
)

// NewLinkResource is the gns3_link resource.
func NewLinkResource() resource.Resource { return &linkResource{} }

type linkResource struct{ client gns3.Client }

type linkResourceModel struct {
	ProjectID types.String         `tfsdk:"project_id"`
	Nodes     []linkEndModel       `tfsdk:"node"`
	Suspend   types.Bool           `tfsdk:"suspend"`
	Filters   jsontypes.Normalized `tfsdk:"filters"`
	LinkID    types.String         `tfsdk:"link_id"`
	LinkType  types.String         `tfsdk:"link_type"`
}

type linkEndModel struct {
	NodeID        types.String `tfsdk:"node_id"`
	AdapterNumber types.Int64  `tfsdk:"adapter_number"`
	PortNumber    types.Int64  `tfsdk:"port_number"`
}

func (r *linkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_link"
}

func (r *linkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A link between two node ports in a GNS3 project. Any change to either `node` block replaces the link.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project UUID. Changing it replaces the link.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"suspend": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Suspend the link (no traffic passes). Default `false`.",
			},
			"filters": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
				MarkdownDescription: "Packet filters as a JSON object (use `jsonencode`), for example " +
					"`jsonencode({ frequency_drop = [50], delay = [100, 10] })`. Removing it clears the filters.",
			},
			"link_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Link UUID assigned by the server.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"link_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`ethernet` or `serial`, set by the server from the ports.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"node": schema.ListNestedBlock{
				MarkdownDescription: "One end of the link. Exactly two blocks.",
				Validators:          []validator.List{listvalidator.SizeBetween(2, 2)},
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"node_id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Node UUID, for example `gns3_node.pc1.node_id`.",
						},
						"adapter_number": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Adapter number, for example `gns3_node.pc1.ports[0].adapter_number`.",
						},
						"port_number": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Port number on the adapter, for example `gns3_node.pc1.ports[0].port_number`.",
						},
					},
				},
			},
		},
	}
}

func (r *linkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func endsToAPI(ends []linkEndModel) []gns3.LinkNode {
	out := make([]gns3.LinkNode, len(ends))
	for i, e := range ends {
		out[i] = gns3.LinkNode{NodeID: e.NodeID.ValueString(), AdapterNumber: e.AdapterNumber.ValueInt64(), PortNumber: e.PortNumber.ValueInt64()}
	}
	return out
}

// sameEnds reports whether a and b hold the same ends in any order.
func sameEnds(a []linkEndModel, b []gns3.LinkNode) bool {
	if len(a) != len(b) {
		return false
	}
	used := make([]bool, len(b))
	for _, e := range a {
		found := false
		for j, n := range b {
			if !used[j] && n.NodeID == e.NodeID.ValueString() && n.AdapterNumber == e.AdapterNumber.ValueInt64() && n.PortNumber == e.PortNumber.ValueInt64() {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// filtersFromAPI keeps a null filters attribute null while the server reports
// no filters ({} or null).
func filtersFromAPI(current jsontypes.Normalized, server json.RawMessage) jsontypes.Normalized {
	empty := len(server) == 0 || string(server) == "null" || string(server) == "{}"
	if empty {
		if current.IsNull() {
			return current
		}
		return jsontypes.NewNormalizedValue("{}")
	}
	return jsontypes.NewNormalizedValue(string(server))
}

func (m *linkResourceModel) fromAPI(l *gns3.Link) {
	m.ProjectID = types.StringValue(l.ProjectID)
	m.LinkID = types.StringValue(l.LinkID)
	m.LinkType = types.StringValue(l.LinkType)
	m.Suspend = types.BoolValue(l.Suspend)
	m.Filters = filtersFromAPI(m.Filters, l.Filters)
	if !sameEnds(m.Nodes, l.Nodes) {
		m.Nodes = make([]linkEndModel, len(l.Nodes))
		for i, n := range l.Nodes {
			m.Nodes[i] = linkEndModel{
				NodeID:        types.StringValue(n.NodeID),
				AdapterNumber: types.Int64Value(n.AdapterNumber),
				PortNumber:    types.Int64Value(n.PortNumber),
			}
		}
	}
}

func filtersToAPI(v jsontypes.Normalized) json.RawMessage {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return json.RawMessage(v.ValueString())
}

func (r *linkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan linkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var suspend *bool
	if plan.Suspend.ValueBool() {
		suspend = knownBool(plan.Suspend)
	}
	l, err := r.client.CreateLink(ctx, plan.ProjectID.ValueString(), gns3.LinkCreateRequest{
		Nodes: endsToAPI(plan.Nodes), Suspend: suspend, Filters: filtersToAPI(plan.Filters),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating GNS3 link", err.Error())
		return
	}
	filters := plan.Filters
	plan.fromAPI(l)
	plan.Filters = filters // keep the configured text; the server may reformat it
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *linkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state linkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID, linkID := state.ProjectID.ValueString(), state.LinkID.ValueString()
	l, err := r.client.GetLink(ctx, projectID, linkID)
	if gns3.IsNotFound(err) {
		if explainMissing(ctx, r.client, projectID, "link "+linkID, &resp.Diagnostics) {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading GNS3 link", err.Error())
		return
	}
	state.fromAPI(l)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *linkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state linkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	filters := filtersToAPI(plan.Filters)
	if filters == nil && !state.Filters.IsNull() {
		filters = json.RawMessage("{}") // removed from config: clear on the server
	}
	l, err := r.client.UpdateLink(ctx, state.ProjectID.ValueString(), state.LinkID.ValueString(), gns3.LinkUpdateRequest{
		Suspend: knownBool(plan.Suspend), Filters: filters,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating GNS3 link", err.Error())
		return
	}
	planned := plan.Filters
	plan.fromAPI(l)
	plan.Filters = planned
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *linkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state linkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteLink(ctx, state.ProjectID.ValueString(), state.LinkID.ValueString())
	if err != nil && !gns3.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting GNS3 link", err.Error())
	}
}

func (r *linkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, linkID, ok := splitImportID(req.ID, "<project_id>/<link_id>", &resp.Diagnostics)
	if !ok {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("link_id"), linkID)...)
}
