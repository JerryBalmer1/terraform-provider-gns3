package gns3

import (
	"context"
	"encoding/json"
	"net/http"
)

// LinksAPI covers /v2/projects/{project_id}/links.
type LinksAPI interface {
	ListLinks(ctx context.Context, projectID string) ([]Link, error)
	GetLink(ctx context.Context, projectID, linkID string) (*Link, error)
	CreateLink(ctx context.Context, projectID string, req LinkCreateRequest) (*Link, error)
	UpdateLink(ctx context.Context, projectID, linkID string, req LinkUpdateRequest) (*Link, error)
	DeleteLink(ctx context.Context, projectID, linkID string) error
}

// Link is the subset of the link object the provider manages.
type Link struct {
	LinkID    string          `json:"link_id"`
	ProjectID string          `json:"project_id"`
	LinkType  string          `json:"link_type"` // ethernet or serial
	Nodes     []LinkNode      `json:"nodes"`
	Suspend   bool            `json:"suspend"`
	Filters   json.RawMessage `json:"filters"`
}

// LinkNode is one end of a link.
type LinkNode struct {
	NodeID        string `json:"node_id"`
	AdapterNumber int64  `json:"adapter_number"`
	PortNumber    int64  `json:"port_number"`
}

// LinkCreateRequest is the POST /v2/projects/{project_id}/links body.
type LinkCreateRequest struct {
	Nodes   []LinkNode      `json:"nodes"`
	Suspend *bool           `json:"suspend,omitempty"`
	Filters json.RawMessage `json:"filters,omitempty"`
}

// LinkUpdateRequest is the PUT /v2/projects/{project_id}/links/{link_id} body.
// Filters set to {} clears them.
type LinkUpdateRequest struct {
	Suspend *bool           `json:"suspend,omitempty"`
	Filters json.RawMessage `json:"filters,omitempty"`
}

func linksPath(projectID string) string {
	return "/v2/projects/" + pathEscape(projectID) + "/links"
}

func linkPath(projectID, linkID string) string {
	return linksPath(projectID) + "/" + pathEscape(linkID)
}

// ListLinks is GET /v2/projects/{project_id}/links.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/link/projectsprojectidlinks.html
func (c *v2Client) ListLinks(ctx context.Context, projectID string) ([]Link, error) {
	var out []Link
	if err := c.do(ctx, http.MethodGet, linksPath(projectID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetLink is GET /v2/projects/{project_id}/links/{link_id}.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/link/projectsprojectidlinkslinkid.html
func (c *v2Client) GetLink(ctx context.Context, projectID, linkID string) (*Link, error) {
	var out Link
	if err := c.do(ctx, http.MethodGet, linkPath(projectID, linkID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateLink is POST /v2/projects/{project_id}/links (201).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/link/projectsprojectidlinks.html
func (c *v2Client) CreateLink(ctx context.Context, projectID string, req LinkCreateRequest) (*Link, error) {
	var out Link
	if err := c.do(ctx, http.MethodPost, linksPath(projectID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateLink is PUT /v2/projects/{project_id}/links/{link_id} (documented as 201).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/link/projectsprojectidlinkslinkid.html
func (c *v2Client) UpdateLink(ctx context.Context, projectID, linkID string, req LinkUpdateRequest) (*Link, error) {
	var out Link
	if err := c.do(ctx, http.MethodPut, linkPath(projectID, linkID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteLink is DELETE /v2/projects/{project_id}/links/{link_id} (204).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/link/projectsprojectidlinkslinkid.html
func (c *v2Client) DeleteLink(ctx context.Context, projectID, linkID string) error {
	return c.do(ctx, http.MethodDelete, linkPath(projectID, linkID), nil, nil)
}
