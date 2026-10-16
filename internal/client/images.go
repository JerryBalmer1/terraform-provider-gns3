package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ImagesAPI covers the emulator images on a compute, through the controller's
// /v2/computes/{compute_id}/{emulator}/... routes. GNS3 2.2 has no endpoint
// that deletes an image (DELETE answers 405 on 2.2.61, docs/api-drift.md).
type ImagesAPI interface {
	ListImages(ctx context.Context, computeID, emulator string) ([]Image, error)
	UploadImage(ctx context.Context, computeID, emulator, filename string, content io.Reader) error
	ListDockerImages(ctx context.Context, computeID string) ([]string, error)
}

// Image is one entry of a compute's image list for qemu, iou or dynamips.
// Path is relative to the emulator's images folder (QEMU, IOU or IOS) when
// the file is inside it, and absolute otherwise; Filename is its base name.
type Image struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
	MD5Sum   string `json:"md5sum"`
	Filesize int64  `json:"filesize"`
}

func imagesPath(computeID, emulator string) string {
	return "/v2/computes/" + pathEscape(computeID) + "/" + pathEscape(emulator) + "/images"
}

// ListImages is GET /v2/computes/{compute_id}/{emulator}/images. emulator is
// qemu, iou or dynamips. A compute without that emulator (iou on Windows)
// answers 404 "GET /iou/images not found". On 2.2.61 the qemu list also
// carries non-ELF files from the other images folders, with absolute paths
// (docs/api-drift.md).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/compute/sidemulatorimages.html
func (c *v2Client) ListImages(ctx context.Context, computeID, emulator string) ([]Image, error) {
	var out []Image
	if err := c.do(ctx, http.MethodGet, imagesPath(computeID, emulator), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UploadImage is POST /v2/computes/{compute_id}/{emulator}/images/{filename}
// with the raw file as the body. The controller forwards it to
// POST /v2/compute/{emulator}/images/{filename} on the compute
// (https://gns3-server.readthedocs.io/en/stable/api/v2/compute/qemu/qemuimagesfilename.html,
// documented as 204); 2.2.61 answers 200 {} and overwrites an existing file.
// The upload is not bound by Config.Timeout, only by ctx, because an image can
// take longer than one API call should.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/compute/sidemulatoraction.html
func (c *v2Client) UploadImage(ctx context.Context, computeID, emulator, filename string, content io.Reader) error {
	segs := strings.Split(filename, "/")
	for i, s := range segs {
		segs[i] = pathEscape(s)
	}
	endpoint := imagesPath(computeID, emulator) + "/" + strings.Join(segs, "/")

	size := int64(-1)
	if f, ok := content.(interface{ Stat() (os.FileInfo, error) }); ok {
		fi, err := f.Stat()
		if err != nil {
			return fmt.Errorf("GNS3 API POST %s: reading image size: %w", endpoint, err)
		}
		size = fi.Size()
	}
	if content == nil {
		content, size = strings.NewReader(""), 0
	}
	upload := *c.httpClient
	upload.Timeout = 0
	_, _, err := c.sendStream(ctx, &upload, http.MethodPost, endpoint, content, size, "application/octet-stream", "")
	return err
}

// ListDockerImages is GET /v2/computes/{compute_id}/docker/images, forwarded
// to GET /v2/compute/docker/images
// (https://gns3-server.readthedocs.io/en/stable/api/v2/compute/docker/dockerimages.html).
// It returns the repo:tag names. A compute without Docker (any Windows
// compute) answers 404 "GET /docker/images not found".
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/compute/sidemulatoraction.html
func (c *v2Client) ListDockerImages(ctx context.Context, computeID string) ([]string, error) {
	var out []struct {
		Image string `json:"image"`
	}
	if err := c.do(ctx, http.MethodGet, imagesPath(computeID, "docker"), nil, &out); err != nil {
		return nil, err
	}
	names := make([]string, len(out))
	for i, img := range out {
		names[i] = img.Image
	}
	return names, nil
}
