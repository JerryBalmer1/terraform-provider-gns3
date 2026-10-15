// Package provider implements the gns3 Terraform provider on
// terraform-plugin-framework.
package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/internal/client"
)

const (
	defaultHost   = "127.0.0.1"
	defaultPort   = 3080
	defaultScheme = "http"
)

var _ provider.Provider = (*gns3Provider)(nil)

type gns3Provider struct {
	version string
	// newClient builds the API client; tests can replace it.
	newClient func(client.Config) client.Client
}

// New returns a provider factory for providerserver.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &gns3Provider{version: version, newClient: client.NewV2}
	}
}

type providerModel struct {
	Host               types.String `tfsdk:"host"`
	Port               types.Int64  `tfsdk:"port"`
	Scheme             types.String `tfsdk:"scheme"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	Token              types.String `tfsdk:"token"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
}

func (p *gns3Provider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "gns3"
	resp.Version = p.version
}

func (p *gns3Provider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages projects, nodes and links on a GNS3 server through its REST API. " +
			"This release talks to GNS3 server 2.2.x over the v2 API (`/v2/...`). " +
			"Each argument falls back to its environment variable, then to its default.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "GNS3 server host name or address. Environment: `GNS3_HOST`. Default `127.0.0.1`.",
			},
			"port": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "GNS3 server port. Environment: `GNS3_PORT`. Default `3080`.",
			},
			"scheme": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "`http` or `https`. Default `http`.",
				Validators:          []validator.String{stringvalidator.OneOf("http", "https")},
			},
			"username": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User for HTTP Basic auth (the `user` setting under `[Server]` with `auth = True` in `gns3_server.conf`). Environment: `GNS3_USERNAME`.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password for HTTP Basic auth. Environment: `GNS3_PASSWORD`.",
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "JWT bearer token for the GNS3 3.x v3 API. Environment: `GNS3_TOKEN`. " +
					"Accepted for forward compatibility; this release speaks only the v2 API and ignores it, with a warning.",
			},
			"insecure_skip_verify": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Skip TLS certificate verification when `scheme` is `https`. Default `false`.",
			},
		},
	}
}

// resolveConfig applies config value, then environment variable, then
// default, for every provider argument. Unknown values are errors.
func resolveConfig(m providerModel, getenv func(string) string, diags *diag.Diagnostics) client.Config {
	unknown := func(attr, env string) {
		diags.AddAttributeError(path.Root(attr), "Unknown GNS3 provider argument",
			fmt.Sprintf("%s is not known until apply. Set it to a literal or use the %s environment variable.", attr, env))
	}
	str := func(v types.String, attr, env, def string) string {
		if v.IsUnknown() {
			unknown(attr, env)
			return ""
		}
		if !v.IsNull() {
			return v.ValueString()
		}
		if env != "" {
			if e := getenv(env); e != "" {
				return e
			}
		}
		return def
	}

	cfg := client.Config{
		Host:     str(m.Host, "host", "GNS3_HOST", defaultHost),
		Scheme:   str(m.Scheme, "scheme", "", defaultScheme),
		Username: str(m.Username, "username", "GNS3_USERNAME", ""),
		Password: str(m.Password, "password", "GNS3_PASSWORD", ""),
		Token:    str(m.Token, "token", "GNS3_TOKEN", ""),
		Port:     defaultPort,
	}
	switch {
	case m.Port.IsUnknown():
		unknown("port", "GNS3_PORT")
	case !m.Port.IsNull():
		cfg.Port = m.Port.ValueInt64()
	default:
		if e := getenv("GNS3_PORT"); e != "" {
			port, err := strconv.ParseInt(e, 10, 64)
			if err != nil {
				diags.AddAttributeError(path.Root("port"), "Invalid GNS3_PORT",
					fmt.Sprintf("GNS3_PORT=%q is not an integer: %v", e, err))
			} else {
				cfg.Port = port
			}
		}
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		diags.AddAttributeError(path.Root("port"), "Invalid GNS3 port", fmt.Sprintf("port %d is outside 1-65535.", cfg.Port))
	}
	if cfg.Scheme != "http" && cfg.Scheme != "https" {
		diags.AddAttributeError(path.Root("scheme"), "Invalid GNS3 scheme", fmt.Sprintf("scheme %q must be http or https.", cfg.Scheme))
	}
	switch {
	case m.InsecureSkipVerify.IsUnknown():
		unknown("insecure_skip_verify", "")
	case !m.InsecureSkipVerify.IsNull():
		cfg.InsecureSkipVerify = m.InsecureSkipVerify.ValueBool()
	}
	if cfg.Password != "" && cfg.Username == "" {
		diags.AddAttributeError(path.Root("username"), "GNS3 password without username",
			"password (or GNS3_PASSWORD) is set but username (or GNS3_USERNAME) is not.")
	}
	return cfg
}

func (p *gns3Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg := resolveConfig(m, os.Getenv, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg.UserAgent = "terraform-provider-gns3/" + p.version
	if cfg.Token != "" {
		resp.Diagnostics.AddAttributeWarning(path.Root("token"), "GNS3 token ignored",
			"token (or GNS3_TOKEN) is for the GNS3 3.x v3 API. This provider release speaks the v2 API "+
				"(GNS3 server 2.2.x), which uses username and password, so the token is not sent.")
	}

	c := p.newClient(cfg)
	v, err := c.GetVersion(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot reach the GNS3 server",
			fmt.Sprintf("Checked %s with GET /v2/version: %v\n\nThis provider release targets GNS3 server 2.2.x (API v2).", cfg.BaseURL(), err))
		return
	}
	if !strings.HasPrefix(v.Version, "2.") {
		resp.Diagnostics.AddWarning("Untested GNS3 server version",
			fmt.Sprintf("%s reports GNS3 %s. This provider release was built against GNS3 server 2.2.52 (API v2).", cfg.BaseURL(), v.Version))
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *gns3Provider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewNodeResource,
		NewLinkResource,
	}
}

func (p *gns3Provider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTemplateDataSource,
		NewComputeDataSource,
		NewProjectDataSource,
		NewVersionDataSource,
	}
}

// clientFrom extracts the client handed over by Configure. data is nil
// before Configure runs (validation); callers then return without error.
func clientFrom(data any, diags *diag.Diagnostics) client.Client {
	if data == nil {
		return nil
	}
	c, ok := data.(client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected client.Client, got %T. This is a provider bug.", data))
		return nil
	}
	return c
}

// splitImportID splits "<a>/<b>" import IDs.
func splitImportID(id, want string, diags *diag.Diagnostics) (string, string, bool) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		diags.AddError("Invalid import ID", fmt.Sprintf("import ID %q must be %s.", id, want))
		return "", "", false
	}
	return parts[0], parts[1], true
}

// explainMissing is called when a node or link GET returns 404. A closed
// project hides its nodes and links, so a 404 there is not proof the object
// is gone; that case is an error instead of a silent removal from state.
// It returns true when the object really is gone.
func explainMissing(ctx context.Context, c client.Client, projectID, what string, diags *diag.Diagnostics) bool {
	p, err := c.GetProject(ctx, projectID)
	if client.IsNotFound(err) {
		return true
	}
	if err != nil {
		diags.AddError(fmt.Sprintf("Error reading GNS3 project for %s", what), err.Error())
		return false
	}
	if p.Status == "closed" {
		diags.AddError(fmt.Sprintf("GNS3 project %s is closed", projectID),
			fmt.Sprintf("The %s could not be read because project %q (%s) is closed. Open it in GNS3, "+
				"or set auto_open = true on the gns3_project so it opens when the server starts.", what, p.Name, projectID))
		return false
	}
	return true
}

func stringOrNull(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

func knownString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func knownInt64(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

func knownBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}
