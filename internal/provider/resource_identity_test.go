package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// identityConfig has one of each managed resource: a project, a vpcs
// template, two nodes from it, a link between them and an image.
func identityConfig(name, imageSource string) string {
	return fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %[1]q
}

resource "gns3_template" "pc" {
  name          = %[1]q
  template_type = "vpcs"
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.test.project_id
  template_id = gns3_template.pc.template_id
  name        = "PC1"
}

resource "gns3_node" "pc2" {
  project_id  = gns3_project.test.project_id
  template_id = gns3_template.pc.template_id
  name        = "PC2"
  x           = 200
}

resource "gns3_link" "test" {
  project_id = gns3_project.test.project_id

  node {
    node_id        = gns3_node.pc1.node_id
    adapter_number = gns3_node.pc1.ports[0].adapter_number
    port_number    = gns3_node.pc1.ports[0].port_number
  }

  node {
    node_id        = gns3_node.pc2.node_id
    adapter_number = gns3_node.pc2.ports[0].adapter_number
    port_number    = gns3_node.pc2.ports[0].port_number
  }
}

resource "gns3_image" "test" {
  emulator    = "qemu"
  filename    = "%[1]s.img"
  source_path = %[2]q
}
`, name, imageSource)
}

// identityChecks checks every identity attribute against the state attribute
// of the same name.
func identityChecks() []statecheck.StateCheck {
	ids := map[string][]string{
		"gns3_project.test": {"project_id"},
		"gns3_template.pc":  {"template_id"},
		"gns3_node.pc1":     {"project_id", "node_id"},
		"gns3_link.test":    {"project_id", "link_id"},
		"gns3_image.test":   {"compute_id", "emulator", "filename"},
	}
	var checks []statecheck.StateCheck
	for addr, attrs := range ids {
		shape := map[string]knownvalue.Check{}
		for _, a := range attrs {
			shape[a] = knownvalue.NotNull()
			checks = append(checks, statecheck.ExpectIdentityValueMatchesState(addr, tfjsonpath.New(a)))
		}
		checks = append(checks, statecheck.ExpectIdentity(addr, shape))
	}
	return checks
}

// identityTestCase applies one of each resource, checks their identities, and
// imports each one by identity and by ID in an import block.
func identityTestCase(t *testing.T, env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-identity")
	src := writeTempImage(t, "terraform-provider-gns3 identity image "+name)
	cfg := env.providerBlock + identityConfig(name, src)
	steps := []resource.TestStep{{
		Config:            cfg,
		ConfigStateChecks: append(identityChecks(), statecheck.ExpectIdentityValue("gns3_image.test", tfjsonpath.New("emulator"), knownvalue.StringExact("qemu"))),
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}}
	// gns3_image is imported by identity in TestImageImportStateByIdentity
	// instead: after any import its plan updates source_path, which the
	// configuration sets and import cannot know, and an import block test
	// requires a no-op plan.
	imports := []struct {
		addr   string
		idFunc resource.ImportStateIdFunc
	}{
		{"gns3_project.test", attrFromState("gns3_project.test", "project_id")},
		{"gns3_template.pc", attrFromState("gns3_template.pc", "template_id")},
		{"gns3_node.pc1", attrFromState("gns3_node.pc1", "project_id", "node_id")},
		{"gns3_link.test", attrFromState("gns3_link.test", "project_id", "link_id")},
	}
	for _, imp := range imports {
		steps = append(steps,
			resource.TestStep{
				Config:          cfg,
				ResourceName:    imp.addr,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
			},
			resource.TestStep{
				Config:            cfg,
				ResourceName:      imp.addr,
				ImportState:       true,
				ImportStateKind:   resource.ImportBlockWithID,
				ImportStateIdFunc: imp.idFunc,
			},
		)
	}
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0)},
		CheckDestroy:             resource.ComposeTestCheckFunc(env.checkGone("gns3_template"), env.checkGone("gns3_project")),
		Steps:                    steps,
	}
}

func TestResourceIdentity(t *testing.T) {
	resource.UnitTest(t, identityTestCase(t, unitEnv(t)))
}

func TestAccResourceIdentity(t *testing.T) {
	resource.Test(t, identityTestCase(t, accEnv(t)))
}

// importByIdentity runs r's ImportState the way the framework does for an
// import block with an identity, and returns the imported state and identity.
func importByIdentity(t *testing.T, r fwresource.ResourceWithIdentity, identity map[string]string) (tfsdk.State, *tfsdk.ResourceIdentity, diag.Diagnostics) {
	t.Helper()
	ctx := context.Background()
	var sr fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &sr)
	var ir fwresource.IdentitySchemaResponse
	r.IdentitySchema(ctx, fwresource.IdentitySchemaRequest{}, &ir)
	vals := map[string]tftypes.Value{}
	for k, v := range identity {
		vals[k] = tftypes.NewValue(tftypes.String, v)
	}
	id := &tfsdk.ResourceIdentity{Schema: ir.IdentitySchema, Raw: tftypes.NewValue(ir.IdentitySchema.Type().TerraformType(ctx), vals)}
	respID := &tfsdk.ResourceIdentity{Schema: ir.IdentitySchema, Raw: id.Raw.Copy()}
	resp := fwresource.ImportStateResponse{
		State:    tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)},
		Identity: respID,
	}
	r.(fwresource.ResourceWithImportState).ImportState(ctx, fwresource.ImportStateRequest{Identity: id}, &resp)
	return resp.State, resp.Identity, resp.Diagnostics
}

// TestImageImportStateByIdentity imports gns3_image from an identity, which
// TestResourceIdentity cannot do through an import block (see there).
func TestImageImportStateByIdentity(t *testing.T) {
	ctx := context.Background()
	state, id, diags := importByIdentity(t, &imageResource{}, map[string]string{
		"compute_id": "vm", "emulator": "dynamips", "filename": "c7200.image",
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	var m imageResourceModel
	if d := state.Get(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	if m.ComputeID.ValueString() != "vm" || m.Emulator.ValueString() != "dynamips" || m.Filename.ValueString() != "c7200.image" {
		t.Errorf("imported state = %s/%s/%s, want vm/dynamips/c7200.image", m.ComputeID, m.Emulator, m.Filename)
	}
	var im imageIdentityModel
	if d := id.Get(ctx, &im); d.HasError() {
		t.Fatal(d)
	}
	if im.Filename.ValueString() != "c7200.image" {
		t.Errorf("identity filename = %s, want c7200.image", im.Filename)
	}

	for name, bad := range map[string]map[string]string{
		"unknown emulator":   {"compute_id": "local", "emulator": "docker", "filename": "x.img"},
		"path in filename":   {"compute_id": "local", "emulator": "qemu", "filename": "a/b.img"},
		"missing compute_id": {"compute_id": "", "emulator": "qemu", "filename": "b.img"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, diags := importByIdentity(t, &imageResource{}, bad)
			if !diags.HasError() || diags.Errors()[0].Summary() != "Invalid import identity" {
				t.Fatalf("diags = %v, want Invalid import identity", diags)
			}
		})
	}
}
