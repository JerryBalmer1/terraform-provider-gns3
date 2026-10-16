package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var fixtureImage = Image{
	Filename: "tf-fixture.img", Path: "tf-fixture.img", MD5Sum: "98d43fc50eb1481391aeb0ffcd1a8f25", Filesize: 37,
}

func TestListImages(t *testing.T) {
	got, err := replay(t, "images/list").ListImages(context.Background(), "local", "qemu")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 17 {
		t.Fatalf("got %d images, want 17", len(got))
	}
	if got[16] != fixtureImage {
		t.Errorf("last image = %+v, want %+v", got[16], fixtureImage)
	}
	if got[0].Filename != "OVMF_CODE.fd" || got[0].Filesize != 3653632 || got[0].MD5Sum == "" {
		t.Errorf("first image = %+v", got[0])
	}
}

func TestListImagesEmpty(t *testing.T) {
	got, err := replay(t, "images/list_empty").ListImages(context.Background(), "local", "dynamips")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty, non-nil slice", got)
	}
}

func TestUploadImage(t *testing.T) {
	err := replay(t, "images/upload").UploadImage(context.Background(), "local", "qemu", "tf-fixture.img",
		strings.NewReader("terraform-provider-gns3 fixture image"))
	if err != nil {
		t.Fatal(err)
	}
}

// An *os.File body is streamed with its length, not chunked.
func TestUploadImageFromFileSendsLength(t *testing.T) {
	content := "terraform-provider-gns3 fixture image"
	p := filepath.Join(t.TempDir(), "tf-fixture.img")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var gotLen int64
	var gotTE []string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLen, gotTE = r.ContentLength, r.TransferEncoding
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()
	if err := NewV2(configFor(t, srv)).UploadImage(context.Background(), "local", "qemu", "tf-fixture.img", f); err != nil {
		t.Fatal(err)
	}
	if gotLen != int64(len(content)) || len(gotTE) != 0 || gotBody != content {
		t.Fatalf("Content-Length %d, Transfer-Encoding %v, body %q", gotLen, gotTE, gotBody)
	}
}

// An upload error names the endpoint and status but never quotes the image.
func TestUploadImageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "POST /iou/images/x.bin not found", "status": 404}`))
	}))
	defer srv.Close()
	err := NewV2(configFor(t, srv)).UploadImage(context.Background(), "local", "iou", "x.bin", strings.NewReader("binary"))
	want := "GNS3 API POST /v2/computes/local/iou/images/x.bin returned HTTP 404: POST /iou/images/x.bin not found"
	if err == nil || err.Error() != want || !IsNotFound(err) {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestListDockerImages(t *testing.T) {
	got, err := replay(t, "images/docker_list").ListDockerImages(context.Background(), "vm")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"alpine:latest", "frrouting/frr:v8.4.0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
