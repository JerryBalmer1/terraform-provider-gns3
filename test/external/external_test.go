package external

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

// TestImportFromOtherModule uses the exported client from a different module:
// constructor, Config, one call, and the exported error helpers.
func TestImportFromOtherModule(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/version" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "not found", "status": 404})
			return
		}
		_ = json.NewEncoder(w).Encode(gns3.Version{Version: "2.2.61", Local: true})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.ParseInt(u.Port(), 10, 64)
	var c gns3.Client = gns3.NewV2(gns3.Config{Host: u.Hostname(), Port: port})

	if got := c.APIVersion(); got != "v2" {
		t.Fatalf("APIVersion() = %q, want v2", got)
	}
	v, err := c.GetVersion(context.Background())
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if v.Version != "2.2.61" {
		t.Fatalf("Version = %q, want 2.2.61", v.Version)
	}
	if _, err := c.GetProject(context.Background(), "missing"); !gns3.IsNotFound(err) {
		t.Fatalf("GetProject(missing) error = %v, want a 404 APIError", err)
	}
}
