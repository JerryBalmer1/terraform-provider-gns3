package provider

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

var (
	_ resource.Resource                     = (*imageResource)(nil)
	_ resource.ResourceWithConfigure        = (*imageResource)(nil)
	_ resource.ResourceWithImportState      = (*imageResource)(nil)
	_ resource.ResourceWithConfigValidators = (*imageResource)(nil)
	_ resource.ResourceWithModifyPlan       = (*imageResource)(nil)
)

// NewImageResource is the gns3_image resource.
func NewImageResource() resource.Resource { return &imageResource{} }

type imageResource struct{ client gns3.Client }

type imageResourceModel struct {
	ComputeID  types.String `tfsdk:"compute_id"`
	Emulator   types.String `tfsdk:"emulator"`
	Filename   types.String `tfsdk:"filename"`
	SourceURL  types.String `tfsdk:"source_url"`
	SourcePath types.String `tfsdk:"source_path"`
	MD5Sum     types.String `tfsdk:"md5sum"`
	Path       types.String `tfsdk:"path"`
	Filesize   types.Int64  `tfsdk:"filesize"`
}

func (r *imageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image"
}

func (r *imageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An emulator image (qemu disk or ISO, IOU binary, Dynamips IOS image) on a GNS3 compute. " +
			"The provider reads the file from `source_url` or `source_path`, checks `md5sum` when it is set, and uploads it " +
			"with `POST /v2/computes/{compute_id}/{emulator}/images/{filename}`. An image already on the compute with the " +
			"same MD5 is adopted without an upload.\n\n" +
			"**Destroy leaves the file on the compute.** GNS3 2.2 has no API to delete an image, so destroying this resource " +
			"only removes it from Terraform state and warns. Delete the file on the compute's file system if you no longer need it.\n\n" +
			"Docker images are not covered: GNS3 pulls them itself when a Docker node is created.",
		Attributes: map[string]schema.Attribute{
			"compute_id": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(defaultComputeID),
				MarkdownDescription: "Compute to upload to, for example `local` or `vm`. Default `local`. Changing it replaces the resource " +
					"(the copy on the old compute stays there).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"emulator": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Emulator whose images folder receives the file: one of `" + joinTicks(imageEmulators) + "`. " +
					"`iou` exists only on Linux computes such as the GNS3 VM. The `dynamips` and `iou` lists show only ELF " +
					"executables, so other files upload but are never found. Changing it replaces the resource.",
				Validators:    []validator.String{stringvalidator.OneOf(imageEmulators...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"filename": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "File name on the compute, at the top of the emulator's images folder (no `/` or `\\`). " +
					"This is the name templates refer to. Changing it replaces the resource (the old file stays).",
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[^/\\]+$`), "must be a file name without / or \\"),
					stringvalidator.NoneOf(".", ".."),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"source_url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "`http` or `https` URL to download the image from, to a temporary file that is removed afterwards. " +
					"Exactly one of `source_url` and `source_path` must be set. Changing it alone does nothing; change `md5sum` too to upload the new file.",
				Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^https?://`), "must be an http:// or https:// URL")},
			},
			"source_path": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Local path, on the machine running Terraform, of the image to upload. Exactly one of `source_url` " +
					"and `source_path` must be set. Changing it alone does nothing; change `md5sum` too to upload the new file.",
			},
			"md5sum": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Expected MD5 of the image, lower-case hex. When set, the source is checked against it after the " +
					"download and before the upload, and the apply fails on a mismatch. Changing it uploads the source again, " +
					"overwriting the file; so does a refresh that finds a different MD5 on the compute. When unset, it is the MD5 " +
					"the compute reports and the file's content is not managed after the first upload.",
				Validators:    []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[0-9a-f]{32}$`), "must be 32 lower-case hex digits")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Path the compute reports, relative to the emulator's images folder (`QEMU`, `IOU` or `IOS`); the file name itself.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"filesize": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "File size in bytes, as the compute reports it.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *imageResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("source_url"), path.MatchRoot("source_path")),
	}
}

// ModifyPlan marks filesize unknown when md5sum changes, because the update
// uploads a different file.
func (r *imageResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state imageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.MD5Sum.Equal(state.MD5Sum) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("filesize"), types.Int64Unknown())...)
	}
}

func (r *imageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (m *imageResourceModel) fromAPI(img gns3.Image) {
	m.Path = types.StringValue(img.Path)
	m.MD5Sum = types.StringValue(img.MD5Sum)
	m.Filesize = types.Int64Value(img.Filesize)
}

// stagedImage is the source file, open at offset 0, with its MD5.
type stagedImage struct {
	file *os.File
	md5  string
	temp bool // file is a download to remove on close
}

func (s *stagedImage) Close() {
	name := s.file.Name()
	_ = s.file.Close()
	if s.temp {
		_ = os.Remove(name)
	}
}

// stageImage opens source_path, or downloads source_url to a temporary file,
// and hashes it.
func stageImage(ctx context.Context, m imageResourceModel) (*stagedImage, error) {
	if p := knownString(m.SourcePath); p != nil {
		f, err := os.Open(*p)
		if err != nil {
			return nil, fmt.Errorf("opening source_path: %w", err)
		}
		s := &stagedImage{file: f}
		if err := s.hash(io.Discard); err != nil {
			s.Close()
			return nil, fmt.Errorf("reading source_path %s: %w", *p, err)
		}
		return s, nil
	}
	u := knownString(m.SourceURL)
	if u == nil {
		return nil, fmt.Errorf("neither source_url nor source_path is set")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *u, nil)
	if err != nil {
		return nil, fmt.Errorf("source_url %s: %w", *u, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading source_url: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("downloading source_url: GET %s returned HTTP %s", *u, resp.Status)
	}
	f, err := os.CreateTemp("", "gns3-image-*")
	if err != nil {
		return nil, fmt.Errorf("creating a temporary file for source_url: %w", err)
	}
	s := &stagedImage{file: f, temp: true}
	h := md5.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		s.Close()
		return nil, fmt.Errorf("downloading source_url %s to %s: %w", *u, f.Name(), err)
	}
	s.md5 = hex.EncodeToString(h.Sum(nil))
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *stagedImage) hash(extra io.Writer) error {
	h := md5.New()
	if _, err := io.Copy(io.MultiWriter(h, extra), s.file); err != nil {
		return err
	}
	s.md5 = hex.EncodeToString(h.Sum(nil))
	_, err := s.file.Seek(0, io.SeekStart)
	return err
}

// sourceName is source_url or source_path, for messages.
func sourceName(m imageResourceModel) string {
	if p := knownString(m.SourcePath); p != nil {
		return "source_path " + *p
	}
	return "source_url " + m.SourceURL.ValueString()
}

// upload stages the source and checks it against md5sum when set. When the
// compute already has the file with the source's MD5 it returns that without
// uploading; with another MD5 it refuses. Otherwise it uploads and returns the
// image as the compute then lists it.
func (r *imageResource) upload(ctx context.Context, m imageResourceModel, want *string, existing *gns3.Image) (gns3.Image, error) {
	computeID, emulator, filename := m.ComputeID.ValueString(), m.Emulator.ValueString(), m.Filename.ValueString()
	src, err := stageImage(ctx, m)
	if err != nil {
		return gns3.Image{}, err
	}
	defer src.Close()
	if want != nil && src.md5 != *want {
		return gns3.Image{}, fmt.Errorf("MD5 mismatch: %s has MD5 %s, md5sum is %s. Nothing was uploaded", sourceName(m), src.md5, *want)
	}
	if existing != nil {
		if existing.MD5Sum == src.md5 {
			return *existing, nil
		}
		return gns3.Image{}, fmt.Errorf("compute %q already has a %s image %q with MD5 %s, and %s has MD5 %s. "+
			"The provider does not overwrite a file it did not upload: import it (terraform import ... %s/%s/%s), "+
			"choose another filename, or remove the file from the compute",
			computeID, emulator, filename, existing.MD5Sum, sourceName(m), src.md5, computeID, emulator, filename)
	}
	if err := r.client.UploadImage(ctx, computeID, emulator, filename, src.file); err != nil {
		return gns3.Image{}, err
	}
	return r.readBack(ctx, computeID, emulator, filename, src.md5)
}

// readBack lists the images after an upload and checks the server's MD5.
func (r *imageResource) readBack(ctx context.Context, computeID, emulator, filename, md5sum string) (gns3.Image, error) {
	images, err := r.client.ListImages(ctx, computeID, emulator)
	if err != nil {
		return gns3.Image{}, fmt.Errorf("listing images after the upload: %w", err)
	}
	img, ok := findImage(images, filename, true)
	if !ok {
		return gns3.Image{}, fmt.Errorf("the upload succeeded but compute %q does not list %q among its %s images (GET %s). %s",
			computeID, filename, emulator, imagesEndpoint(computeID, emulator), listFilterHint(emulator))
	}
	if img.MD5Sum != md5sum {
		return gns3.Image{}, fmt.Errorf("after the upload compute %q reports MD5 %s for %q, but the uploaded file has MD5 %s",
			computeID, img.MD5Sum, filename, md5sum)
	}
	return img, nil
}

func (r *imageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan imageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	computeID, emulator, filename := plan.ComputeID.ValueString(), plan.Emulator.ValueString(), plan.Filename.ValueString()
	want := knownString(plan.MD5Sum)

	images, err := r.client.ListImages(ctx, computeID, emulator)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error listing %s images on GNS3 compute %q", emulator, computeID), err.Error())
		return
	}
	var existing *gns3.Image
	if img, ok := findImage(images, filename, true); ok {
		existing = &img
	}

	var img gns3.Image
	if existing != nil && want != nil && existing.MD5Sum == *want {
		img = *existing // already there: adopt without reading the source
	} else if img, err = r.upload(ctx, plan, want, existing); err != nil {
		resp.Diagnostics.AddError("Error uploading GNS3 image", err.Error())
		return
	}
	plan.fromAPI(img)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *imageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state imageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	computeID, emulator, filename := state.ComputeID.ValueString(), state.Emulator.ValueString(), state.Filename.ValueString()
	images, err := r.client.ListImages(ctx, computeID, emulator)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error listing %s images on GNS3 compute %q", emulator, computeID), err.Error())
		return
	}
	img, ok := findImage(images, filename, true)
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}
	state.fromAPI(img)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *imageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state imageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	want := knownString(plan.MD5Sum)
	if want == nil || plan.MD5Sum.Equal(state.MD5Sum) {
		// Only source_url or source_path changed: recorded, nothing uploaded.
		plan.Path, plan.MD5Sum, plan.Filesize = state.Path, state.MD5Sum, state.Filesize
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	// md5sum changed (in the configuration, or on the compute since the last
	// refresh): upload the source over the file this resource owns.
	src, err := stageImage(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error uploading GNS3 image", err.Error())
		return
	}
	defer src.Close()
	if src.md5 != *want {
		resp.Diagnostics.AddAttributeError(path.Root("md5sum"), "Error uploading GNS3 image",
			fmt.Sprintf("MD5 mismatch: %s has MD5 %s, md5sum is %s. Nothing was uploaded; the file on the compute is unchanged.",
				sourceName(plan), src.md5, *want))
		return
	}
	computeID, emulator, filename := plan.ComputeID.ValueString(), plan.Emulator.ValueString(), plan.Filename.ValueString()
	if err := r.client.UploadImage(ctx, computeID, emulator, filename, src.file); err != nil {
		resp.Diagnostics.AddError("Error uploading GNS3 image", err.Error())
		return
	}
	img, err := r.readBack(ctx, computeID, emulator, filename, src.md5)
	if err != nil {
		resp.Diagnostics.AddError("Error uploading GNS3 image", err.Error())
		return
	}
	plan.fromAPI(img)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the image: GNS3 2.2 has no endpoint that deletes one
// (DELETE /v2/computes/{compute_id}/{emulator}/images/{filename} answers 405
// on 2.2.61, docs/api-drift.md).
func (r *imageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state imageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.AddWarning("GNS3 image left on the compute",
		fmt.Sprintf("GNS3 2.2 has no API to delete images, so %q stays in the %s images folder on compute %q (path %s). "+
			"It was removed from Terraform state only. Delete the file on the compute's file system if you no longer need it.",
			state.Filename.ValueString(), state.Emulator.ValueString(), state.ComputeID.ValueString(), state.Path.ValueString()))
}

func (r *imageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[2] == "" || !slices.Contains(imageEmulators, parts[1]) || strings.ContainsAny(parts[2], `/\`) {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("import ID %q must be <compute_id>/<emulator>/<filename>, with emulator one of %s and a file name without / or \\.",
				req.ID, strings.Join(imageEmulators, ", ")))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("compute_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("emulator"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("filename"), parts[2])...)
}
