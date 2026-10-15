package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fixture is one request/response pair under testdata/<area>/<name>.json.
// _header states provenance: "hand-written from docs, not recorded" or
// "recorded from GNS3 <version> on <date>".
type fixture struct {
	Header  string `json:"_header"`
	Source  string `json:"_source"`
	Request struct {
		Method string          `json:"method"`
		Path   string          `json:"path"`
		Body   json.RawMessage `json:"body"`
		Text   *string         `json:"text"` // raw non-JSON body (node files)
	} `json:"request"`
	Response struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
		Text   *string         `json:"text"` // raw non-JSON body (node files)
	} `json:"response"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(name)+".json"))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parsing fixture %s: %v", name, err)
	}
	if f.Header != "hand-written from docs, not recorded" && !strings.HasPrefix(f.Header, "recorded from GNS3 ") {
		t.Fatalf("fixture %s: _header %q does not state its provenance", name, f.Header)
	}
	if f.Source == "" {
		t.Fatalf("fixture %s: _source is empty", name)
	}
	return f
}

// replay starts an httptest server that answers the named fixtures in order,
// failing the test if a request's method, path or JSON body differs from the
// fixture. It returns a v2 client pointed at the server.
func replay(t *testing.T, names ...string) Client {
	t.Helper()
	_, cfg := replayServer(t, names...)
	return NewV2(cfg)
}

func replayServer(t *testing.T, names ...string) (*httptest.Server, Config) {
	t.Helper()
	fixtures := make([]fixture, len(names))
	for i, n := range names {
		fixtures[i] = loadFixture(t, n)
	}
	var mu sync.Mutex
	next := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if next >= len(fixtures) {
			t.Errorf("unexpected request %s %s after all %d fixtures were used", r.Method, r.URL.Path, len(fixtures))
			http.Error(w, "no fixture", http.StatusInternalServerError)
			return
		}
		f, name := fixtures[next], names[next]
		next++
		if r.Method != f.Request.Method || r.URL.Path != f.Request.Path {
			t.Errorf("fixture %s: got %s %s, want %s %s", name, r.Method, r.URL.Path, f.Request.Method, f.Request.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if want := f.Request.Body; len(want) > 0 && string(want) != "null" {
			if !jsonEqual(t, body, want) {
				t.Errorf("fixture %s: request body\n got: %s\nwant: %s", name, body, want)
			}
		}
		if want := f.Request.Text; want != nil && string(body) != *want {
			t.Errorf("fixture %s: request text\n got: %q\nwant: %q", name, body, *want)
		}
		if f.Response.Text != nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(f.Response.Status)
			_, _ = w.Write([]byte(*f.Response.Text))
			return
		}
		if len(f.Response.Body) > 0 && string(f.Response.Body) != "null" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.Response.Status)
			_, _ = w.Write(f.Response.Body)
			return
		}
		w.WriteHeader(f.Response.Status)
	}))
	t.Cleanup(func() {
		srv.Close()
		if next != len(fixtures) {
			t.Errorf("only %d of %d fixtures were requested", next, len(fixtures))
		}
	})
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.ParseInt(u.Port(), 10, 64)
	return srv, Config{Scheme: "http", Host: u.Hostname(), Port: port}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Errorf("request body is not JSON: %v: %s", err, a)
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Errorf("fixture body is not JSON: %v", err)
		return false
	}
	return reflect.DeepEqual(av, bv)
}

func ptr[T any](v T) *T { return &v }
