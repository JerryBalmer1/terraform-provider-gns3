package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// imageDataSourceTestCase uploads a small generated file with gns3_image, so
// there is a known image to look up, then reads it back with the data source.
// Against a live server the file stays in its QEMU images folder: GNS3 2.2
// cannot delete images.
func imageDataSourceTestCase(t *testing.T, env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-image-ds") + ".img"
	content := "terraform-provider-gns3 data source test " + name
	src := writeTempImage(t, content)
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + fmt.Sprintf(`
resource "gns3_image" "test" {
  emulator    = "qemu"
  filename    = %q
  source_path = %q
}

data "gns3_image" "test" {
  emulator = "qemu"
  filename = gns3_image.test.filename
}
`, name, src),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.gns3_image.test", "compute_id", "local"),
					resource.TestCheckResourceAttr("data.gns3_image.test", "path", name),
					resource.TestCheckResourceAttr("data.gns3_image.test", "md5sum", md5Hex(content)),
					resource.TestCheckResourceAttr("data.gns3_image.test", "filesize", fmt.Sprint(len(content))),
					resource.TestCheckResourceAttrPair("data.gns3_image.test", "md5sum", "gns3_image.test", "md5sum"),
				),
			},
			{
				Config: env.providerBlock + `
data "gns3_image" "missing" {
  compute_id = "local"
  emulator   = "qemu"
  filename   = "no-such-image-tfacc.qcow2"
}
`,
				ExpectError: regexp.MustCompile(`(?s)GNS3 image not found.*Compute "local" lists no qemu image named\s+"no-such-image-tfacc.qcow2"`),
			},
		},
	}
}

func TestImageDataSource(t *testing.T) {
	resource.UnitTest(t, imageDataSourceTestCase(t, unitEnv(t)))
}

// An image copied onto the compute by hand, not uploaded by Terraform.
func TestImageDataSourceExistingFile(t *testing.T) {
	env := unitEnv(t)
	env.fake.putImage("qemu", "vyos-rolling.iso", []byte("not really an ISO"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{{
			Config: env.providerBlock + `
data "gns3_image" "vyos" {
  emulator = "qemu"
  filename = "vyos-rolling.iso"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.gns3_image.vyos", "md5sum", md5Hex("not really an ISO")),
				resource.TestCheckResourceAttr("data.gns3_image.vyos", "filesize", "17"),
			),
		}},
	})
}

// The not-found message for dynamips explains its ELF filter; a compute
// without IOU passes the server's 404 through.
func TestImageDataSourceErrors(t *testing.T) {
	env := unitEnv(t)
	env.fake.putImage("dynamips", "c7200.image", []byte("not an ELF file"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config:      env.providerBlock + "data \"gns3_image\" \"ios\" {\n  emulator = \"dynamips\"\n  filename = \"c7200.image\"\n}\n",
				ExpectError: regexp.MustCompile(`(?s)lists no dynamips image.*32-bit big-endian ELF`),
			},
			{
				Config:      env.providerBlock + "data \"gns3_image\" \"iou\" {\n  emulator = \"iou\"\n  filename = \"i86bi.bin\"\n}\n",
				ExpectError: wrapped("GNS3 API GET /v2/computes/local/iou/images returned HTTP 404: GET /iou/images not found"),
			},
			{
				Config:      env.providerBlock + "data \"gns3_image\" \"bad\" {\n  emulator = \"docker\"\n  filename = \"x\"\n}\n",
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestAccImageDataSource(t *testing.T) {
	resource.Test(t, imageDataSourceTestCase(t, accEnv(t)))
}
