package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/internal/client"
)

var (
	_ datasource.DataSource                     = (*templateDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*templateDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*templateDataSource)(nil)
)

// NewTemplateDataSource is the gns3_template data source.
func NewTemplateDataSource() datasource.DataSource { return &templateDataSource{} }

type templateDataSource struct{ client client.Client }

type templateDataSourceModel struct {
	TemplateID   types.String `tfsdk:"template_id"`
	Name         types.String `tfsdk:"name"`
	TemplateType types.String `tfsdk:"template_type"`
	ComputeID    types.String `tfsdk:"compute_id"`
	Category     types.String `tfsdk:"category"`
	Symbol       types.String `tfsdk:"symbol"`
	Builtin      types.Bool   `tfsdk:"builtin"`
}

func (d *templateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_template"
}

func (d *templateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a GNS3 template by `name` or `template_id`. Exactly one must be set. " +
			"Built-in templates (VPCS, Ethernet switch, Ethernet hub, Cloud, NAT, ...) need no image.",
		Attributes: map[string]schema.Attribute{
			"template_id": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Template UUID.",
			},
			"name": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Template name, for example `VPCS`. Must match exactly one template.",
			},
			"template_type": schema.StringAttribute{Computed: true, MarkdownDescription: "Emulator type, for example `vpcs` or `qemu`."},
			"compute_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "Default compute, null for built-in templates."},
			"category":      schema.StringAttribute{Computed: true, MarkdownDescription: "`router`, `switch`, `guest` or `firewall`."},
			"symbol":        schema.StringAttribute{Computed: true, MarkdownDescription: "Symbol path."},
			"builtin":       schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the template ships with GNS3."},
		},
	}
}

func (d *templateDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("template_id"), path.MatchRoot("name")),
	}
}

func (d *templateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *templateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m templateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var tpl *client.Template
	if !m.TemplateID.IsNull() {
		t, err := d.client.GetTemplate(ctx, m.TemplateID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading GNS3 template", err.Error())
			return
		}
		tpl = t
	} else {
		all, err := d.client.ListTemplates(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error listing GNS3 templates", err.Error())
			return
		}
		name := m.Name.ValueString()
		var matches []client.Template
		for _, t := range all {
			if t.Name == name {
				matches = append(matches, t)
			}
		}
		switch len(matches) {
		case 0:
			resp.Diagnostics.AddAttributeError(path.Root("name"), "GNS3 template not found",
				fmt.Sprintf("No template named %q among the %d templates returned by GET /v2/templates.", name, len(all)))
			return
		case 1:
			tpl = &matches[0]
		default:
			resp.Diagnostics.AddAttributeError(path.Root("name"), "GNS3 template name is ambiguous",
				fmt.Sprintf("%d templates are named %q; use template_id instead.", len(matches), name))
			return
		}
	}
	m.TemplateID = types.StringValue(tpl.TemplateID)
	m.Name = types.StringValue(tpl.Name)
	m.TemplateType = types.StringValue(tpl.TemplateType)
	m.ComputeID = stringOrNull(tpl.ComputeID)
	m.Category = types.StringValue(tpl.Category)
	m.Symbol = types.StringValue(tpl.Symbol)
	m.Builtin = types.BoolValue(tpl.Builtin)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
