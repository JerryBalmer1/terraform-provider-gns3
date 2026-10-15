package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func versionDataSourceTestCase(env testEnv) resource.TestCase {
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{{
			Config: env.providerBlock + "data \"gns3_version\" \"server\" {}\n",
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestMatchResourceAttr("data.gns3_version.server", "version", regexp.MustCompile(`^2\.2\.`)),
				resource.TestCheckResourceAttrSet("data.gns3_version.server", "local"),
				resource.TestCheckResourceAttr("data.gns3_version.server", "api_version", "v2"),
			),
		}},
	}
}

func TestVersionDataSource(t *testing.T) { resource.UnitTest(t, versionDataSourceTestCase(unitEnv(t))) }

func TestAccVersionDataSource(t *testing.T) { resource.Test(t, versionDataSourceTestCase(accEnv(t))) }
