package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ list.ListResource              = (*nodeResource)(nil)
	_ list.ListResourceWithConfigure = (*nodeResource)(nil)
)

// NewNodeListResource is the gns3_node list resource.
func NewNodeListResource() list.ListResource { return &nodeResource{} }

type nodeListConfigModel struct {
	ProjectID types.String `tfsdk:"project_id"`
}

func (r *nodeResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists the nodes of one GNS3 project (`GET /v2/projects/{project_id}/nodes`). " +
			"The project must be open: GNS3 does not report the nodes of a closed project. " +
			"With resources included, `properties` and `startup_config` are null, as after `terraform import`.",
		Attributes: map[string]listschema.Attribute{
			"project_id": listschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the project whose nodes are listed.",
			},
		},
	}
}

// List returns the project's nodes, with the state Read would write after an
// import when the query asks for resources.
func (r *nodeResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var cfg nodeListConfigModel
	diags := req.Config.Get(ctx, &cfg)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	// GET /v2/projects/{project_id}/nodes,
	// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
	nodes, err := r.client.ListNodes(ctx, cfg.ProjectID.ValueString())
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing GNS3 nodes", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	stream.Results = func(push func(list.ListResult) bool) {
		for i := range nodes {
			if listLimitReached(i, req.Limit) {
				return
			}
			n := &nodes[i]
			result := req.NewListResult(ctx)
			result.DisplayName = n.Name
			setNodeIdentity(ctx, result.Identity, types.StringValue(n.ProjectID), types.StringValue(n.NodeID), &result.Diagnostics)
			if req.IncludeResource && result.Resource != nil {
				var m nodeResourceModel
				m.fill(ctx, r.client, n, &result.Diagnostics)
				result.Diagnostics.Append(result.Resource.Set(ctx, &m)...)
			}
			if !push(result) {
				return
			}
		}
	}
}
