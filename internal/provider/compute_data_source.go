package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

var (
	_ datasource.DataSource                     = (*computeDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*computeDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*computeDataSource)(nil)
)

// NewComputeDataSource is the gns3_compute data source.
func NewComputeDataSource() datasource.DataSource { return &computeDataSource{} }

type computeDataSource struct{ client gns3.Client }

type computeDataSourceModel struct {
	ComputeID types.String `tfsdk:"compute_id"`
	Name      types.String `tfsdk:"name"`
	Host      types.String `tfsdk:"host"`
	Port      types.Int64  `tfsdk:"port"`
	Protocol  types.String `tfsdk:"protocol"`
	Connected types.Bool   `tfsdk:"connected"`
}

func (d *computeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute"
}

func (d *computeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a compute registered with the GNS3 controller by `compute_id` or `name`. Exactly one must be set.",
		Attributes: map[string]schema.Attribute{
			"compute_id": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Compute identifier, for example `local` or `vm`.",
			},
			"name": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Compute name. Must match exactly one compute.",
			},
			"host":      schema.StringAttribute{Computed: true, MarkdownDescription: "Compute host."},
			"port":      schema.Int64Attribute{Computed: true, MarkdownDescription: "Compute port."},
			"protocol":  schema.StringAttribute{Computed: true, MarkdownDescription: "`http` or `https`."},
			"connected": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the controller is connected to the compute."},
		},
	}
}

func (d *computeDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("compute_id"), path.MatchRoot("name")),
	}
}

func (d *computeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *computeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m computeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var c *gns3.Compute
	if !m.ComputeID.IsNull() {
		got, err := d.client.GetCompute(ctx, m.ComputeID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading GNS3 compute", err.Error())
			return
		}
		c = got
	} else {
		all, err := d.client.ListComputes(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error listing GNS3 computes", err.Error())
			return
		}
		name := m.Name.ValueString()
		var matches []gns3.Compute
		for _, x := range all {
			if x.Name == name {
				matches = append(matches, x)
			}
		}
		switch len(matches) {
		case 0:
			resp.Diagnostics.AddAttributeError(path.Root("name"), "GNS3 compute not found",
				fmt.Sprintf("No compute named %q among the %d computes returned by GET /v2/computes.", name, len(all)))
			return
		case 1:
			c = &matches[0]
		default:
			resp.Diagnostics.AddAttributeError(path.Root("name"), "GNS3 compute name is ambiguous",
				fmt.Sprintf("%d computes are named %q; use compute_id instead.", len(matches), name))
			return
		}
	}
	m.ComputeID = types.StringValue(c.ComputeID)
	m.Name = types.StringValue(c.Name)
	m.Host = types.StringValue(c.Host)
	m.Port = types.Int64Value(c.Port)
	m.Protocol = types.StringValue(c.Protocol)
	m.Connected = types.BoolValue(c.Connected)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
