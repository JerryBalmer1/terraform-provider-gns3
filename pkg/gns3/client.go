package gns3

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is every GNS3 controller operation the provider uses.
type Client interface {
	ProjectsAPI
	NodesAPI
	LinksAPI
	TemplatesAPI
	ComputesAPI
	ImagesAPI
	VersionAPI

	// APIVersion is the REST API generation this client speaks, "v2" or "v3".
	APIVersion() string
}

// Config holds connection settings shared by every API generation.
type Config struct {
	Scheme             string // "http" or "https"
	Host               string
	Port               int64
	Username           string // v2: HTTP Basic auth user
	Password           string // v2: HTTP Basic auth password
	Token              string // v3: JWT bearer token; unused by v2
	InsecureSkipVerify bool
	UserAgent          string
	Timeout            time.Duration // per request; 0 means 2 minutes
}

// BaseURL is scheme://host:port with no trailing slash.
func (c Config) BaseURL() string {
	scheme := c.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return (&url.URL{Scheme: scheme, Host: net.JoinHostPort(c.Host, strconv.FormatInt(c.Port, 10))}).String()
}

// APIError is a non-2xx response from the server. Error() always names the
// method, the endpoint, the HTTP status and the server's message, then adds
// the request body that was sent and any part of the response body the
// message leaves out.
type APIError struct {
	Method       string
	Endpoint     string // path including /v2 prefix and IDs
	StatusCode   int
	Message      string // "message" from the server's JSON error body, or the raw body
	RequestBody  string // JSON sent, "" when the request had no body
	ResponseBody string // raw body received
}

// maxErrorBody caps each body quoted in Error().
const maxErrorBody = 4096

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	s := fmt.Sprintf("GNS3 API %s %s returned HTTP %d: %s", e.Method, e.Endpoint, e.StatusCode, msg)
	if e.RequestBody != "" && e.RequestBody != "{}" {
		s += "\nRequest body: " + truncate(e.RequestBody)
	}
	if responseAddsDetail(e.ResponseBody, e.Message) {
		s += "\nResponse body: " + truncate(e.ResponseBody)
	}
	return s
}

// responseAddsDetail is false when the body is empty, is the message itself,
// or is a JSON object holding only "message" and "status".
func responseAddsDetail(body, message string) bool {
	body = strings.TrimSpace(body)
	if body == "" || body == message {
		return false
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &obj) != nil {
		return true
	}
	for k := range obj {
		if k != "message" && k != "status" {
			return true
		}
	}
	return false
}

func truncate(s string) string {
	if len(s) <= maxErrorBody {
		return s
	}
	return s[:maxErrorBody] + fmt.Sprintf("... (%d bytes)", len(s))
}

// IsNotFound reports whether err is an APIError with HTTP 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// v2Client implements Client against GNS3 server 2.2.x.
type v2Client struct {
	cfg        Config
	baseURL    string
	httpClient *http.Client
}

var _ Client = (*v2Client)(nil)

// NewV2 returns a Client for the GNS3 2.2.x REST API (/v2). Config.Token is
// ignored; v2 authenticates with HTTP Basic auth when Username is set.
func NewV2(cfg Config) Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &v2Client{
		cfg:        cfg,
		baseURL:    cfg.BaseURL(),
		httpClient: &http.Client{Transport: transport, Timeout: timeout},
	}
}

func (c *v2Client) APIVersion() string { return "v2" }

// do sends one request. body is JSON-encoded when non-nil; out is decoded from
// a 2xx response body when non-nil. Any non-2xx status becomes an *APIError.
func (c *v2Client) do(ctx context.Context, method, endpoint string, body, out any) error {
	var sent []byte
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("GNS3 API %s %s: encoding request body: %w", method, endpoint, err)
		}
		sent = buf
	}
	respBody, status, err := c.send(ctx, method, endpoint, sent, "application/json")
	if err != nil {
		return err
	}
	if out == nil || len(bytes.TrimSpace(respBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("GNS3 API %s %s returned HTTP %d; decoding body: %w", method, endpoint, status, err)
	}
	return nil
}

// send sends one request with a raw body (none when body is nil) and returns
// the 2xx response body. Any non-2xx status becomes an *APIError.
func (c *v2Client) send(ctx context.Context, method, endpoint string, body []byte, contentType string) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	return c.sendStream(ctx, c.httpClient, method, endpoint, reader, -1, contentType, string(body))
}

// sendStream is send with a streamed body. size is the body length, or -1
// when it is unknown or the reader reports it itself; errBody is what an
// APIError quotes as the request body.
func (c *v2Client) sendStream(ctx context.Context, hc *http.Client, method, endpoint string, body io.Reader, size int64, contentType, errBody string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, body)
	if err != nil {
		return nil, 0, fmt.Errorf("GNS3 API %s %s: building request: %w", method, endpoint, err)
	}
	if size >= 0 {
		req.ContentLength = size
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("GNS3 API %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("GNS3 API %s %s returned HTTP %d; reading body: %w", method, endpoint, resp.StatusCode, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, resp.StatusCode, &APIError{
			Method: method, Endpoint: endpoint, StatusCode: resp.StatusCode, Message: errorMessage(respBody),
			RequestBody: errBody, ResponseBody: string(respBody),
		}
	}
	return respBody, resp.StatusCode, nil
}

// errorMessage pulls "message" out of the server's error body
// ({"message": "...", "status": N}, per the 409 sample on
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/link/projectsprojectidlinks.html),
// falling back to the trimmed raw body.
func errorMessage(body []byte) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		return e.Message
	}
	return strings.TrimSpace(string(body))
}

func pathEscape(id string) string { return url.PathEscape(id) }
