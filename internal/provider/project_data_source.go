package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

var (
	_ datasource.DataSource              = (*projectDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*projectDataSource)(nil)
)

// NewProjectDataSource is the gns3_project data source.
func NewProjectDataSource() datasource.DataSource { return &projectDataSource{} }

type projectDataSource struct{ client gns3.Client }

type projectDataSourceModel struct {
	Name      types.String `tfsdk:"name"`
	ProjectID types.String `tfsdk:"project_id"`
	Path      types.String `tfsdk:"path"`
	Filename  types.String `tfsdk:"filename"`
	Status    types.String `tfsdk:"status"`
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing GNS3 project by name, opened or closed.",
		Attributes: map[string]schema.Attribute{
			"name":       schema.StringAttribute{Required: true, MarkdownDescription: "Project name."},
			"project_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Project UUID."},
			"path":       schema.StringAttribute{Computed: true, MarkdownDescription: "Project directory on the server."},
			"filename":   schema.StringAttribute{Computed: true, MarkdownDescription: "Project file name."},
			"status":     schema.StringAttribute{Computed: true, MarkdownDescription: "`opened` or `closed`."},
		},
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m projectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := d.client.ListProjects(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing GNS3 projects", err.Error())
		return
	}
	name := m.Name.ValueString()
	var found *gns3.Project
	for i := range all {
		if all[i].Name == name {
			found = &all[i]
			break
		}
	}
	if found == nil {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "GNS3 project not found",
			fmt.Sprintf("No project named %q among the %d projects returned by GET /v2/projects.", name, len(all)))
		return
	}
	m.ProjectID = types.StringValue(found.ProjectID)
	m.Path = types.StringValue(found.Path)
	m.Filename = types.StringValue(found.Filename)
	m.Status = types.StringValue(found.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
