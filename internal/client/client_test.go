package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestConfigBaseURL(t *testing.T) {
	cases := []struct {
		cfg  Config
		want string
	}{
		{Config{Host: "127.0.0.1", Port: 3080}, "http://127.0.0.1:3080"},
		{Config{Scheme: "https", Host: "gns3.lab", Port: 443}, "https://gns3.lab:443"},
		{Config{Scheme: "http", Host: "::1", Port: 3080}, "http://[::1]:3080"},
	}
	for _, c := range cases {
		if got := c.cfg.BaseURL(); got != c.want {
			t.Errorf("BaseURL(%+v) = %q, want %q", c.cfg, got, c.want)
		}
	}
}

func TestAPIVersion(t *testing.T) {
	if got := NewV2(Config{}).APIVersion(); got != "v2" {
		t.Fatalf("APIVersion() = %q, want v2", got)
	}
}

func TestAPIErrorCarriesStatusEndpointAndMessage(t *testing.T) {
	c := replay(t, "projects/create_409")
	_, err := c.CreateProject(context.Background(), ProjectCreateRequest{Name: "test"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %v is not *APIError", err)
	}
	if apiErr.StatusCode != 409 || apiErr.Endpoint != "/v2/projects" || apiErr.Method != "POST" {
		t.Errorf("got %+v", apiErr)
	}
	want := "GNS3 API POST /v2/projects returned HTTP 409: Project \"test\" already exists\nRequest body: {\"name\":\"test\"}"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
	if IsNotFound(err) {
		t.Error("IsNotFound(409) = true")
	}
}

func TestAPIErrorNonJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream proxy exploded", http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewV2(configFor(t, srv))
	_, err := c.GetVersion(context.Background())
	want := "GNS3 API GET /v2/version returned HTTP 502: upstream proxy exploded"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestAPIErrorEmptyBodyUsesStatusText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := NewV2(configFor(t, srv)).GetVersion(context.Background())
	want := "GNS3 API GET /v2/version returned HTTP 401: Unauthorized"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

// A JSON error body with more than message and status (GNS3 adds "path" and
// "validation" on schema errors) is quoted after the message.
func TestAPIErrorQuotesResponseDetail(t *testing.T) {
	body := `{"message": "JSON schema error", "path": ["name"], "status": 400}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	_, err := NewV2(configFor(t, srv)).CreateProject(context.Background(), ProjectCreateRequest{Name: "x"})
	want := "GNS3 API POST /v2/projects returned HTTP 400: JSON schema error\nRequest body: {\"name\":\"x\"}\nResponse body: " + body
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestTransportErrorNamesEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	cfg := configFor(t, srv)
	srv.Close() // nothing listens on the port now
	_, err := NewV2(cfg).GetVersion(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "GNS3 API GET /v2/version: ") {
		t.Fatalf("err = %v, want prefix naming the endpoint", err)
	}
}

func TestBasicAuthAndUserAgent(t *testing.T) {
	var gotUser, gotPass, gotUA string
	var gotOK bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, gotOK = r.BasicAuth()
		gotUA = r.UserAgent()
		_, _ = w.Write([]byte(`{"version":"2.2.52","local":false}`))
	}))
	defer srv.Close()
	cfg := configFor(t, srv)
	cfg.Username, cfg.Password, cfg.UserAgent = "admin", "s3cret", "terraform-provider-gns3/test"
	cfg.Token = "ignored-by-v2"
	if _, err := NewV2(cfg).GetVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !gotOK || gotUser != "admin" || gotPass != "s3cret" {
		t.Errorf("basic auth = %q/%q ok=%v", gotUser, gotPass, gotOK)
	}
	if gotUA != "terraform-provider-gns3/test" {
		t.Errorf("User-Agent = %q", gotUA)
	}
}

func TestNoAuthHeaderWithoutUsername(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"version":"2.2.52"}`))
	}))
	defer srv.Close()
	cfg := configFor(t, srv)
	cfg.Token = "ignored-by-v2"
	if _, err := NewV2(cfg).GetVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want none", gotAuth)
	}
}

func TestInsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":"2.2.52"}`))
	}))
	defer srv.Close()
	cfg := configFor(t, srv)
	cfg.Scheme = "https"
	if _, err := NewV2(cfg).GetVersion(context.Background()); err == nil {
		t.Fatal("self-signed certificate accepted without insecure_skip_verify")
	}
	cfg.InsecureSkipVerify = true
	if _, err := NewV2(cfg).GetVersion(context.Background()); err != nil {
		t.Fatalf("insecure_skip_verify: %v", err)
	}
}

func configFor(t *testing.T, srv *httptest.Server) Config {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.ParseInt(u.Port(), 10, 64)
	return Config{Scheme: "http", Host: u.Hostname(), Port: port}
}
