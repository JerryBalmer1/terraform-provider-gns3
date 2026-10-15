package provider

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/JerryBalmer1/terraform-provider-gns3/internal/client"
)

var protoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"gns3": providerserver.NewProtocol6WithError(New("test")()),
}

// testEnv is where a test case runs: the in-memory fake (unit tests, always)
// or a live server from GNS3_* environment variables (acceptance, TF_ACC=1).
// Both run the same steps.
type testEnv struct {
	providerBlock string
	client        client.Client
	fake          *fakeGNS3 // nil against a live server
}

func unitEnv(t *testing.T) testEnv {
	t.Helper()
	f := newFakeGNS3(t)
	u, _ := url.Parse(f.srv.URL)
	port, _ := strconv.ParseInt(u.Port(), 10, 64)
	return testEnv{
		providerBlock: f.providerBlock(),
		client:        client.NewV2(client.Config{Scheme: "http", Host: u.Hostname(), Port: port}),
		fake:          f,
	}
}

func accEnv(t *testing.T) testEnv {
	t.Helper()
	var diags diag.Diagnostics
	cfg := resolveConfig(nullProviderModel(), os.Getenv, &diags)
	if diags.HasError() {
		t.Fatalf("resolving GNS3_* environment: %v", diags)
	}
	return testEnv{client: client.NewV2(cfg)}
}

func nullProviderModel() providerModel {
	return providerModel{
		Host: types.StringNull(), Port: types.Int64Null(), Scheme: types.StringNull(),
		Username: types.StringNull(), Password: types.StringNull(), Token: types.StringNull(),
		InsecureSkipVerify: types.BoolNull(),
	}
}

func attrFromState(name string, attrs ...string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("%s not in state", name)
		}
		id := ""
		for i, a := range attrs {
			if i > 0 {
				id += "/"
			}
			id += rs.Primary.Attributes[a]
		}
		return id, nil
	}
}

// checkGone fails if any resource of the given type in state still exists on
// the server.
func (e testEnv) checkGone(resourceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		for _, rs := range s.RootModule().Resources {
			if rs.Type != resourceType {
				continue
			}
			a := rs.Primary.Attributes
			var err error
			switch resourceType {
			case "gns3_project":
				_, err = e.client.GetProject(ctx, a["project_id"])
			case "gns3_node":
				_, err = e.client.GetNode(ctx, a["project_id"], a["node_id"])
			case "gns3_link":
				_, err = e.client.GetLink(ctx, a["project_id"], a["link_id"])
			}
			if err == nil {
				return fmt.Errorf("%s %s still exists", resourceType, rs.Primary.Attributes)
			}
			if !client.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
}

func TestResolveConfigDefaults(t *testing.T) {
	var diags diag.Diagnostics
	cfg := resolveConfig(nullProviderModel(), func(string) string { return "" }, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := client.Config{Scheme: "http", Host: "127.0.0.1", Port: 3080}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestResolveConfigEnvironment(t *testing.T) {
	env := map[string]string{
		"GNS3_HOST": "gns3.lab", "GNS3_PORT": "3443", "GNS3_USERNAME": "admin",
		"GNS3_PASSWORD": "pw", "GNS3_TOKEN": "jwt",
	}
	var diags diag.Diagnostics
	cfg := resolveConfig(nullProviderModel(), func(k string) string { return env[k] }, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := client.Config{Scheme: "http", Host: "gns3.lab", Port: 3443, Username: "admin", Password: "pw", Token: "jwt"}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestResolveConfigValueBeatsEnvironment(t *testing.T) {
	m := nullProviderModel()
	m.Host, m.Port, m.Scheme = types.StringValue("10.0.0.5"), types.Int64Value(8443), types.StringValue("https")
	m.Username, m.Password = types.StringValue("u"), types.StringValue("p")
	m.InsecureSkipVerify = types.BoolValue(true)
	env := map[string]string{"GNS3_HOST": "ignored", "GNS3_PORT": "1", "GNS3_USERNAME": "ignored", "GNS3_PASSWORD": "ignored"}
	var diags diag.Diagnostics
	cfg := resolveConfig(m, func(k string) string { return env[k] }, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := client.Config{Scheme: "https", Host: "10.0.0.5", Port: 8443, Username: "u", Password: "p", InsecureSkipVerify: true}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestResolveConfigErrors(t *testing.T) {
	cases := map[string]struct {
		model providerModel
		env   map[string]string
		want  string
	}{
		"bad env port":      {nullProviderModel(), map[string]string{"GNS3_PORT": "eighty"}, "Invalid GNS3_PORT"},
		"port out of range": {withPort(70000), nil, "Invalid GNS3 port"},
		"password only":     {nullProviderModel(), map[string]string{"GNS3_PASSWORD": "pw"}, "GNS3 password without username"},
		"unknown host":      {withUnknownHost(), nil, "Unknown GNS3 provider argument"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var diags diag.Diagnostics
			resolveConfig(c.model, func(k string) string { return c.env[k] }, &diags)
			if !diags.HasError() || diags.Errors()[0].Summary() != c.want {
				t.Fatalf("diags = %v, want error %q", diags, c.want)
			}
		})
	}
}

func withPort(p int64) providerModel {
	m := nullProviderModel()
	m.Port = types.Int64Value(p)
	return m
}

func withUnknownHost() providerModel {
	m := nullProviderModel()
	m.Host = types.StringUnknown()
	return m
}

func TestProviderUnreachableServer(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{{
			Config:      "provider \"gns3\" {\n  host = \"127.0.0.1\"\n  port = 1\n}\ndata \"gns3_version\" \"v\" {}\n",
			ExpectError: regexp.MustCompile(`(?s)Cannot reach the GNS3 server.*GET /v2/version`),
		}},
	})
}

// exampleDirs lists examples/resources/<name>/ directories.
func exampleDirs(t *testing.T) []string {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join("..", "..", "examples", "resources", "*", "resource.tf"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no examples found: %v", err)
	}
	return dirs
}

// examplesTestCase applies one example and requires an empty plan afterwards.
// resource.Test also runs its own post-apply plan and fails on drift.
func examplesTestCase(env testEnv, file string) resource.TestCase {
	src, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             env.checkGone("gns3_project"),
		Steps: []resource.TestStep{{
			Config: env.providerBlock + string(src),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		}},
	}
}

func TestExamples(t *testing.T) {
	for _, file := range exampleDirs(t) {
		t.Run(filepath.Base(filepath.Dir(file)), func(t *testing.T) {
			resource.UnitTest(t, examplesTestCase(unitEnv(t), file))
		})
	}
}

func TestAccExamples(t *testing.T) {
	for _, file := range exampleDirs(t) {
		t.Run(filepath.Base(filepath.Dir(file)), func(t *testing.T) {
			resource.Test(t, examplesTestCase(accEnv(t), file))
		})
	}
}
