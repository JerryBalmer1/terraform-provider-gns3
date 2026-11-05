package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ list.ListResource              = (*templateResource)(nil)
	_ list.ListResourceWithConfigure = (*templateResource)(nil)
)

// NewTemplateListResource is the gns3_template list resource.
func NewTemplateListResource() list.ListResource { return &templateResource{} }

type templateListConfigModel struct {
	TemplateType   types.String `tfsdk:"template_type"`
	IncludeBuiltin types.Bool   `tfsdk:"include_builtin"`
}

func (r *templateResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists the user templates on the GNS3 controller (`GET /v2/templates`). " +
			"Built-in templates (VPCS, Ethernet switch, ...) are left out unless `include_builtin = true`, because " +
			"`gns3_template` cannot manage or import them. " +
			"With resources included, `properties` is null, as after `terraform import`.",
		Attributes: map[string]listschema.Attribute{
			"template_type": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "List only templates of this type: one of `" + joinTicks(templateTypes) + "`. All types when unset.",
				Validators:          []validator.String{stringvalidator.OneOf(templateTypes...)},
			},
			"include_builtin": listschema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Also list built-in templates (`builtin = true`). Default `false`. " +
					"For looking at what the server has: a built-in result cannot be imported into `gns3_template`.",
			},
		},
	}
}

// List returns the user templates, optionally of one type, with the state
// Read would write after an import when the query asks for resources.
func (r *templateResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var cfg templateListConfigModel
	diags := req.Config.Get(ctx, &cfg)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	// GET /v2/templates, https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/templates.html
	templates, err := r.client.ListTemplates(ctx)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing GNS3 templates", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	includeBuiltin := cfg.IncludeBuiltin.ValueBool() // null is false
	stream.Results = func(push func(list.ListResult) bool) {
		n := 0
		for i := range templates {
			t := &templates[i]
			if (t.Builtin && !includeBuiltin) || (!cfg.TemplateType.IsNull() && t.TemplateType != cfg.TemplateType.ValueString()) {
				continue
			}
			if listLimitReached(n, req.Limit) {
				return
			}
			n++
			result := req.NewListResult(ctx)
			result.DisplayName = t.Name
			setTemplateIdentity(ctx, result.Identity, templateResourceModel{TemplateID: types.StringValue(t.TemplateID)}, &result.Diagnostics)
			if req.IncludeResource && result.Resource != nil {
				var m templateResourceModel
				m.fill(t, &result.Diagnostics)
				result.Diagnostics.Append(result.Resource.Set(ctx, &m)...)
			}
			if !push(result) {
				return
			}
		}
	}
}
