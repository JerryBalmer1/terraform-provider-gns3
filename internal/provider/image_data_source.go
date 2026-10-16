package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/internal/client"
)

var (
	_ datasource.DataSource              = (*imageDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*imageDataSource)(nil)
)

// imageEmulators are the {emulator} values GET /v2/computes/{compute_id}/{emulator}/images
// serves (https://gns3-server.readthedocs.io/en/stable/api/v2/controller/compute/sidemulatorimages.html).
// Each is accepted by the controller; a compute without the emulator (iou on
// Windows) answers 404 (seen on 2.2.61, docs/api-drift.md).
var imageEmulators = []string{"qemu", "iou", "dynamips"}

// NewImageDataSource is the gns3_image data source.
func NewImageDataSource() datasource.DataSource { return &imageDataSource{} }

type imageDataSource struct{ client client.Client }

type imageDataSourceModel struct {
	ComputeID types.String `tfsdk:"compute_id"`
	Emulator  types.String `tfsdk:"emulator"`
	Filename  types.String `tfsdk:"filename"`
	Path      types.String `tfsdk:"path"`
	MD5Sum    types.String `tfsdk:"md5sum"`
	Filesize  types.Int64  `tfsdk:"filesize"`
}

func (d *imageDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image"
}

func (d *imageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an emulator image already on a compute, by exact file name, through " +
			"`GET /v2/computes/{compute_id}/{emulator}/images`. Fails when the compute does not list the file. " +
			"Read-only: it never uploads or changes anything. Docker images are not covered; GNS3 pulls those itself.",
		Attributes: map[string]schema.Attribute{
			"compute_id": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Compute to look on, for example `local` or `vm`. Default `local`.",
			},
			"emulator": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Emulator whose images folder to search: one of `" + joinTicks(imageEmulators) + "`. " +
					"`iou` exists only on Linux computes such as the GNS3 VM.",
				Validators: []validator.String{stringvalidator.OneOf(imageEmulators...)},
			},
			"filename": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Exact file name, for example `vyos-rolling.iso`. A file in a subfolder or in another images folder " +
					"is found by its base name; `path` says where it is.",
			},
			"path": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Path the compute reports: relative to the emulator's images folder (`QEMU`, `IOU` or `IOS`) " +
					"when the file is inside it, absolute otherwise.",
			},
			"md5sum":   schema.StringAttribute{Computed: true, MarkdownDescription: "MD5 checksum the compute reports, lower-case hex."},
			"filesize": schema.Int64Attribute{Computed: true, MarkdownDescription: "File size in bytes."},
		},
	}
}

func (d *imageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *imageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m imageDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	computeID := defaultComputeID
	if c := knownString(m.ComputeID); c != nil {
		computeID = *c
	}
	emulator, filename := m.Emulator.ValueString(), m.Filename.ValueString()
	images, err := d.client.ListImages(ctx, computeID, emulator)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error listing %s images on GNS3 compute %q", emulator, computeID), err.Error())
		return
	}
	img, ok := findImage(images, filename, false)
	if !ok {
		resp.Diagnostics.AddAttributeError(path.Root("filename"), "GNS3 image not found",
			fmt.Sprintf("Compute %q lists no %s image named %q among its %d %s images (GET %s). %s",
				computeID, emulator, filename, len(images), emulator, imagesEndpoint(computeID, emulator), listFilterHint(emulator)))
		return
	}
	m.ComputeID = types.StringValue(computeID)
	m.Path = types.StringValue(img.Path)
	m.MD5Sum = types.StringValue(img.MD5Sum)
	m.Filesize = types.Int64Value(img.Filesize)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func imagesEndpoint(computeID, emulator string) string {
	return "/v2/computes/" + computeID + "/" + emulator + "/images"
}

// findImage finds filename in a compute's image list. The server lists each
// file name once, the first one found winning (list_images in
// gns3server/utils/images.py, 2.2 branch). topLevel also requires the file to
// sit at the top of the emulator's images folder, where an upload puts it:
// its path is then the bare file name.
func findImage(images []client.Image, filename string, topLevel bool) (client.Image, bool) {
	for _, img := range images {
		if img.Filename == filename && (!topLevel || img.Path == filename) {
			return img, true
		}
	}
	return client.Image{}, false
}

// listFilterHint names the files an image list leaves out
// (list_images in gns3server/utils/images.py, 2.2 branch; the dynamips case
// seen on 2.2.61, docs/api-drift.md).
func listFilterHint(emulator string) string {
	switch emulator {
	case "dynamips":
		return "The dynamips list shows only 32-bit big-endian ELF files (unpacked IOS images) of 7 bytes or more."
	case "iou":
		return "The iou list shows only little-endian ELF files (IOU binaries) of 7 bytes or more."
	default:
		return "The qemu list leaves out ELF executables and files under 7 bytes."
	}
}
