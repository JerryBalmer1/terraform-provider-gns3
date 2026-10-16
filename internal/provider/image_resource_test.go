package provider

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// wrapped matches literal text that Terraform may have wrapped across lines.
func wrapped(text string) *regexp.Regexp {
	return regexp.MustCompile(strings.Join(strings.Fields(regexp.QuoteMeta(text)), `\s+`))
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// writeTempImage writes a generated "image" (any non-ELF file of 7 bytes or
// more lists as qemu) and returns its path with forward slashes for HCL.
func writeTempImage(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "image.img")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(p)
}

func imageConfig(name, source, md5sum string) string {
	md5Line := ""
	if md5sum != "" {
		md5Line = fmt.Sprintf("  md5sum      = %q\n", md5sum)
	}
	return fmt.Sprintf(`
resource "gns3_image" "test" {
  emulator    = "qemu"
  filename    = %q
  source_path = %q
%s}
`, name, source, md5Line)
}

// imageTestCase uploads a generated file, shows that a new source alone
// changes nothing, re-uploads when md5sum changes, imports, and rejects a
// source that does not match md5sum. Against a live server the file stays in
// its QEMU images folder after destroy: GNS3 2.2 cannot delete images.
func imageTestCase(t *testing.T, env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-image") + ".img"
	a := "terraform-provider-gns3 image A " + name
	b := "terraform-provider-gns3 image B, a little longer " + name
	srcA, srcB := writeTempImage(t, a), writeTempImage(t, b)
	const addr = "gns3_image.test"
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + imageConfig(name, srcA, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "compute_id", "local"),
					resource.TestCheckResourceAttr(addr, "path", name),
					resource.TestCheckResourceAttr(addr, "md5sum", md5Hex(a)),
					resource.TestCheckResourceAttr(addr, "filesize", fmt.Sprint(len(a))),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// A new source without a new md5sum is recorded but not uploaded.
				Config: env.providerBlock + imageConfig(name, srcB, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "source_path", srcB),
					resource.TestCheckResourceAttr(addr, "md5sum", md5Hex(a)),
				),
			},
			{
				Config: env.providerBlock + imageConfig(name, srcB, md5Hex(b)),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue(addr, tfjsonpath.New("filesize")),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "md5sum", md5Hex(b)),
					resource.TestCheckResourceAttr(addr, "filesize", fmt.Sprint(len(b))),
				),
			},
			{
				ResourceName:                         addr,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        "local/qemu/" + name,
				ImportStateVerifyIdentifierAttribute: "filename",
				ImportStateVerifyIgnore:              []string{"source_path", "source_url"},
			},
			{
				Config:      env.providerBlock + imageConfig(name, srcB, md5Hex(a)),
				ExpectError: wrapped("has MD5 " + md5Hex(b) + ", md5sum is " + md5Hex(a) + ". Nothing was uploaded"),
			},
		},
	}
}

func TestImageResource(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, imageTestCase(t, env))
	if n := env.fake.imageUploads(); n != 2 {
		t.Errorf("fake received %d uploads, want 2 (create, md5sum change)", n)
	}
	if n := len(env.fake.images["qemu"]); n != 1 {
		t.Errorf("fake holds %d qemu images after destroy, want 1: destroy must not delete the file", n)
	}
}

func TestAccImageResource(t *testing.T) {
	resource.Test(t, imageTestCase(t, accEnv(t)))
}

// imageURLTestCase downloads the source from an HTTP server in the test
// process, which works the same against the fake and a live GNS3 server.
func imageURLTestCase(t *testing.T, env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-image-url") + ".img"
	content := "terraform-provider-gns3 downloaded image " + name
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+name {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(srv.Close)
	config := func(url, md5sum string) string {
		return env.providerBlock + fmt.Sprintf(`
resource "gns3_image" "test" {
  compute_id = "local"
  emulator   = "qemu"
  filename   = %q
  source_url = %q
  md5sum     = %q
}
`, name, url, md5sum)
	}
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config:      config(srv.URL+"/missing.img", md5Hex(content)),
				ExpectError: wrapped("downloading source_url: GET " + srv.URL + "/missing.img returned HTTP 404 Not Found"),
			},
			{
				Config:      config(srv.URL+"/"+name, md5Hex("something else")),
				ExpectError: wrapped("MD5 mismatch: source_url " + srv.URL + "/" + name + " has MD5 " + md5Hex(content)),
			},
			{
				Config: config(srv.URL+"/"+name, md5Hex(content)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_image.test", "md5sum", md5Hex(content)),
					resource.TestCheckResourceAttr("gns3_image.test", "filesize", fmt.Sprint(len(content))),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	}
}

func TestImageResourceSourceURL(t *testing.T) {
	resource.UnitTest(t, imageURLTestCase(t, unitEnv(t)))
}

func TestAccImageResourceSourceURL(t *testing.T) {
	resource.Test(t, imageURLTestCase(t, accEnv(t)))
}

// A file already on the compute with the same MD5 is adopted, with or
// without md5sum, and never uploaded again.
func TestImageResourceAdoptsExistingFile(t *testing.T) {
	env := unitEnv(t)
	content := "terraform-provider-gns3 adopted image"
	env.fake.putImage("qemu", "a.img", []byte(content))
	env.fake.putImage("qemu", "b.img", []byte(content))
	src := writeTempImage(t, content)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{{
			Config: env.providerBlock + fmt.Sprintf(`
resource "gns3_image" "with_md5" {
  emulator    = "qemu"
  filename    = "a.img"
  source_url  = "http://127.0.0.1:1/never-fetched.img"
  md5sum      = %q
}

resource "gns3_image" "same_content" {
  emulator    = "qemu"
  filename    = "b.img"
  source_path = %q
}
`, md5Hex(content), src),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("gns3_image.with_md5", "md5sum", md5Hex(content)),
				resource.TestCheckResourceAttr("gns3_image.same_content", "md5sum", md5Hex(content)),
				func(*terraform.State) error {
					if n := env.fake.imageUploads(); n != 0 {
						return fmt.Errorf("%d uploads, want 0", n)
					}
					return nil
				},
			),
		}},
	})
}

// A different file of the same name is never overwritten at create.
func TestImageResourceRefusesToOverwrite(t *testing.T) {
	env := unitEnv(t)
	env.fake.putImage("qemu", "taken.img", []byte("somebody else's image"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{{
			Config:      env.providerBlock + imageConfig("taken.img", writeTempImage(t, "my image content"), ""),
			ExpectError: wrapped(`compute "local" already has a qemu image "taken.img" with MD5 ` + md5Hex("somebody else's image")),
		}},
	})
	if n := env.fake.imageUploads(); n != 0 {
		t.Errorf("%d uploads, want 0", n)
	}
}

// A file removed from the compute leaves state and is uploaded again; a file
// changed on the compute is uploaded again when md5sum is set.
func TestImageResourceDrift(t *testing.T) {
	env := unitEnv(t)
	content := "terraform-provider-gns3 drift image"
	src := writeTempImage(t, content)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + imageConfig("drift.img", src, md5Hex(content)),
				Check: func(*terraform.State) error {
					env.fake.removeImage("qemu", "drift.img")
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
			{
				Config: env.providerBlock + imageConfig("drift.img", src, md5Hex(content)),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("gns3_image.test", plancheck.ResourceActionCreate)},
				},
				Check: func(*terraform.State) error {
					env.fake.putImage("qemu", "drift.img", []byte("changed on the compute"))
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
			{
				Config: env.providerBlock + imageConfig("drift.img", src, md5Hex(content)),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction("gns3_image.test", plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("gns3_image.test", "md5sum", md5Hex(content)),
			},
		},
	})
	if n := env.fake.imageUploads(); n != 3 {
		t.Errorf("%d uploads, want 3", n)
	}
}

func TestImageResourceErrors(t *testing.T) {
	env := unitEnv(t)
	src := writeTempImage(t, "terraform-provider-gns3 error image")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config:      env.providerBlock + imageConfig("sub/x.img", src, ""),
				ExpectError: regexp.MustCompile(`must be a file name without / or`),
			},
			{
				Config: env.providerBlock + fmt.Sprintf(`
resource "gns3_image" "test" {
  emulator    = "qemu"
  filename    = "x.img"
  source_path = %q
  source_url  = "https://example.com/x.img"
}
`, src),
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
			},
			{
				Config:      env.providerBlock + imageConfig("x.img", src, "ABC"),
				ExpectError: regexp.MustCompile(`must be 32 lower-case hex digits`),
			},
			{
				// Uploads, but the dynamips list only shows big-endian ELF files.
				Config: env.providerBlock + fmt.Sprintf(`
resource "gns3_image" "test" {
  emulator    = "dynamips"
  filename    = "c7200.image"
  source_path = %q
}
`, src),
				ExpectError: regexp.MustCompile(`(?s)the upload succeeded but compute "local" does not list\s+"c7200.image".*32-bit big-endian ELF`),
			},
			{
				Config:        env.providerBlock + imageConfig("x.img", src, ""),
				ResourceName:  "gns3_image.test",
				ImportState:   true,
				ImportStateId: "local/docker/x.img",
				ExpectError:   regexp.MustCompile(`must be\s+<compute_id>/<emulator>/<filename>`),
			},
		},
	})
}
