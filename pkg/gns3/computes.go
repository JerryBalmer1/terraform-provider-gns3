package gns3

import (
	"context"
	"net/http"
)

// ComputesAPI covers /v2/computes (read-only in 0.1.0).
type ComputesAPI interface {
	ListComputes(ctx context.Context) ([]Compute, error)
	GetCompute(ctx context.Context, computeID string) (*Compute, error)
}

// Compute is a compute server registered with the controller.
type Compute struct {
	ComputeID string  `json:"compute_id"`
	Name      string  `json:"name"`
	Host      string  `json:"host"`
	Port      int64   `json:"port"`
	Protocol  string  `json:"protocol"` // http or https
	User      *string `json:"user"`
	Connected bool    `json:"connected"`
}

// ListComputes is GET /v2/computes.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/compute/s.html
func (c *v2Client) ListComputes(ctx context.Context) ([]Compute, error) {
	var out []Compute
	if err := c.do(ctx, http.MethodGet, "/v2/computes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCompute is GET /v2/computes/{compute_id}.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/compute/sid.html
func (c *v2Client) GetCompute(ctx context.Context, computeID string) (*Compute, error) {
	var out Compute
	if err := c.do(ctx, http.MethodGet, "/v2/computes/"+pathEscape(computeID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
