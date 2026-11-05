package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

// nodeListTestCase creates two nodes in a project made outside Terraform,
// because the query file needs the project UUID as a literal, then lists them
// with and without resources.
func nodeListTestCase(t *testing.T, env testEnv) resource.TestCase {
	ctx := context.Background()
	p, err := env.client.CreateProject(ctx, gns3.ProjectCreateRequest{Name: acctest.RandomWithPrefix("tf-acc-list-node")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.client.CloseProject(ctx, p.ProjectID); err == nil {
			_ = deleteProjectRetrying(ctx, env.client, p.ProjectID)
		}
	})
	cfg := env.providerBlock + fmt.Sprintf(`
data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc1" {
  project_id  = %[1]q
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC1"
}

resource "gns3_node" "pc2" {
  project_id  = %[1]q
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC2"
  x           = 200
}
`, p.ProjectID)
	pc2 := queryfilter.ByDisplayName(knownvalue.StringExact("PC2"))
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		TerraformVersionChecks:   listQueryVersion,
		CheckDestroy:             env.checkGone("gns3_node"),
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Query: true,
				Config: fmt.Sprintf(`
list "gns3_node" "ids" {
  provider = gns3
  config {
    project_id = %[1]q
  }
}

list "gns3_node" "full" {
  provider         = gns3
  include_resource = true
  config {
    project_id = %[1]q
  }
}
`, p.ProjectID),
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("gns3_node.ids", 2),
					querycheck.ExpectIdentity("gns3_node.ids", map[string]knownvalue.Check{
						"project_id": knownvalue.StringExact(p.ProjectID), "node_id": knownvalue.NotNull(),
					}),
					querycheck.ExpectResourceDisplayName("gns3_node.ids", pc2, knownvalue.StringExact("PC2")),
					querycheck.ExpectLength("gns3_node.full", 2),
					querycheck.ExpectResourceKnownValues("gns3_node.full", pc2, []querycheck.KnownValueCheck{
						{Path: tfjsonpath.New("project_id"), KnownValue: knownvalue.StringExact(p.ProjectID)},
						{Path: tfjsonpath.New("x"), KnownValue: knownvalue.Int64Exact(200)},
						{Path: tfjsonpath.New("node_type"), KnownValue: knownvalue.StringExact("vpcs")},
						{Path: tfjsonpath.New("start_on_create"), KnownValue: knownvalue.Bool(false)},
						{Path: tfjsonpath.New("properties"), KnownValue: knownvalue.Null()},
						{Path: tfjsonpath.New("ports"), KnownValue: knownvalue.ListSizeExact(1)},
					}),
				},
			},
		},
	}
}

// TestNodeList also checks that a failed GET reaches the query output with
// the server's message.
func TestNodeList(t *testing.T) {
	tc := nodeListTestCase(t, unitEnv(t))
	tc.Steps = append(tc.Steps, resource.TestStep{
		Query: true,
		Config: `
list "gns3_node" "missing" {
  provider = gns3
  config {
    project_id = "00000000-0000-0000-0000-000000000000"
  }
}
`,
		ExpectError: regexp.MustCompile(`(?s)Error listing GNS3 nodes.*GET /v2/projects/00000000-0000-0000-0000-000000000000/nodes.*404`),
	})
	resource.UnitTest(t, tc)
}

func TestAccNodeList(t *testing.T) {
	// nodeListTestCase calls the server before resource.Test can skip.
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("acceptance test: set %s=1", resource.EnvTfAcc)
	}
	resource.Test(t, nodeListTestCase(t, accEnv(t)))
}
