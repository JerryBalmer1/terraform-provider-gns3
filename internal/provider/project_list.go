package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
)

var (
	_ list.ListResource              = (*projectResource)(nil)
	_ list.ListResourceWithConfigure = (*projectResource)(nil)
)

// NewProjectListResource is the gns3_project list resource.
func NewProjectListResource() list.ListResource { return &projectResource{} }

func (r *projectResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists every project on the GNS3 controller, opened or closed (`GET /v2/projects`). Takes no arguments.",
	}
}

// List returns every project, with the state Read would write when the query
// asks for resources.
func (r *projectResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	// GET /v2/projects, https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projects.html
	projects, err := r.client.ListProjects(ctx)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing GNS3 projects", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	stream.Results = func(push func(list.ListResult) bool) {
		for i := range projects {
			if listLimitReached(i, req.Limit) {
				return
			}
			p := &projects[i]
			result := req.NewListResult(ctx)
			result.DisplayName = p.Name
			var m projectResourceModel
			m.fromAPI(p)
			setProjectIdentity(ctx, result.Identity, m, &result.Diagnostics)
			if req.IncludeResource && result.Resource != nil {
				result.Diagnostics.Append(result.Resource.Set(ctx, &m)...)
			}
			if !push(result) {
				return
			}
		}
	}
}
