package client

import (
	"context"
	"net/http"
)

// VersionAPI covers /v2/version.
type VersionAPI interface {
	GetVersion(ctx context.Context) (*Version, error)
}

// Version is the server version and whether it runs in --local mode.
type Version struct {
	Version string `json:"version"`
	Local   bool   `json:"local"`
}

// GetVersion is GET /v2/version.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/server/version.html
func (c *v2Client) GetVersion(ctx context.Context) (*Version, error) {
	var out Version
	if err := c.do(ctx, http.MethodGet, "/v2/version", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
