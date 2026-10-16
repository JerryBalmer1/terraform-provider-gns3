package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

var (
	_ datasource.DataSource              = (*versionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*versionDataSource)(nil)
)

// NewVersionDataSource is the gns3_version data source.
func NewVersionDataSource() datasource.DataSource { return &versionDataSource{} }

type versionDataSource struct{ client gns3.Client }

type versionDataSourceModel struct {
	Version    types.String `tfsdk:"version"`
	Local      types.Bool   `tfsdk:"local"`
	APIVersion types.String `tfsdk:"api_version"`
}

func (d *versionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_version"
}

func (d *versionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The GNS3 server version.",
		Attributes: map[string]schema.Attribute{
			"version":     schema.StringAttribute{Computed: true, MarkdownDescription: "Server version, for example `2.2.52`."},
			"local":       schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the server runs with `--local` (local paths allowed)."},
			"api_version": schema.StringAttribute{Computed: true, MarkdownDescription: "REST API generation the provider uses, `v2` in this release."},
		},
	}
}

func (d *versionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *versionDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	v, err := d.client.GetVersion(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading GNS3 server version", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &versionDataSourceModel{
		Version:    types.StringValue(v.Version),
		Local:      types.BoolValue(v.Local),
		APIVersion: types.StringValue(d.client.APIVersion()),
	})...)
}
