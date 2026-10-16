package gns3

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// NodesAPI covers /v2/projects/{project_id}/nodes and node creation from a
// template.
type NodesAPI interface {
	ListNodes(ctx context.Context, projectID string) ([]Node, error)
	GetNode(ctx context.Context, projectID, nodeID string) (*Node, error)
	CreateNode(ctx context.Context, projectID string, req NodeCreateRequest) (*Node, error)
	CreateNodeFromTemplate(ctx context.Context, projectID, templateID string, req NodeFromTemplateRequest) (*Node, error)
	UpdateNode(ctx context.Context, projectID, nodeID string, req NodeUpdateRequest) (*Node, error)
	DeleteNode(ctx context.Context, projectID, nodeID string) error
	StartNode(ctx context.Context, projectID, nodeID string) (*Node, error)
	StopNode(ctx context.Context, projectID, nodeID string) (*Node, error)
	ReadNodeFile(ctx context.Context, projectID, nodeID, path string) ([]byte, error)
	WriteNodeFile(ctx context.Context, projectID, nodeID, path string, content []byte) error
}

// Node is the subset of the node object the provider manages.
type Node struct {
	NodeID      string                     `json:"node_id"`
	ProjectID   string                     `json:"project_id"`
	ComputeID   string                     `json:"compute_id"`
	TemplateID  *string                    `json:"template_id"`
	NodeType    string                     `json:"node_type"`
	Name        string                     `json:"name"`
	Status      string                     `json:"status"` // stopped, started, suspended
	Symbol      *string                    `json:"symbol"`
	Console     *int64                     `json:"console"`
	ConsoleType *string                    `json:"console_type"`
	ConsoleHost string                     `json:"console_host"`
	X           int64                      `json:"x"`
	Y           int64                      `json:"y"`
	Z           int64                      `json:"z"`
	Ports       []Port                     `json:"ports"`
	Properties  map[string]json.RawMessage `json:"properties"`
}

// Port is one entry of a node's read-only "ports" list.
type Port struct {
	AdapterNumber int64  `json:"adapter_number"`
	PortNumber    int64  `json:"port_number"`
	Name          string `json:"name"`
	ShortName     string `json:"short_name"`
	LinkType      string `json:"link_type"`
}

// NodeCreateRequest is the POST /v2/projects/{project_id}/nodes body.
type NodeCreateRequest struct {
	ComputeID   string                     `json:"compute_id"`
	Name        string                     `json:"name"`
	NodeType    string                     `json:"node_type"`
	ConsoleType *string                    `json:"console_type,omitempty"`
	Symbol      *string                    `json:"symbol,omitempty"`
	X           *int64                     `json:"x,omitempty"`
	Y           *int64                     `json:"y,omitempty"`
	Z           *int64                     `json:"z,omitempty"`
	Properties  map[string]json.RawMessage `json:"properties,omitempty"`
}

// NodeFromTemplateRequest is the POST /v2/projects/{project_id}/templates/{template_id}
// body. x and y are mandatory; compute_id is used when the template has no
// default compute (built-in templates have compute_id null).
type NodeFromTemplateRequest struct {
	X         int64   `json:"x"`
	Y         int64   `json:"y"`
	Name      *string `json:"name,omitempty"`
	ComputeID *string `json:"compute_id,omitempty"`
}

// NodeUpdateRequest is the PUT /v2/projects/{project_id}/nodes/{node_id} body.
// The docs sample sends compute_id, name and node_type alongside the changed
// fields, so callers fill those with the node's current values.
type NodeUpdateRequest struct {
	ComputeID   string                     `json:"compute_id"`
	Name        string                     `json:"name"`
	NodeType    string                     `json:"node_type"`
	ConsoleType *string                    `json:"console_type,omitempty"`
	Symbol      *string                    `json:"symbol,omitempty"`
	X           *int64                     `json:"x,omitempty"`
	Y           *int64                     `json:"y,omitempty"`
	Z           *int64                     `json:"z,omitempty"`
	Properties  map[string]json.RawMessage `json:"properties,omitempty"`
}

func nodesPath(projectID string) string {
	return "/v2/projects/" + pathEscape(projectID) + "/nodes"
}

func nodePath(projectID, nodeID string) string {
	return nodesPath(projectID) + "/" + pathEscape(nodeID)
}

// ListNodes is GET /v2/projects/{project_id}/nodes.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
func (c *v2Client) ListNodes(ctx context.Context, projectID string) ([]Node, error) {
	var out []Node
	if err := c.do(ctx, http.MethodGet, nodesPath(projectID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetNode is GET /v2/projects/{project_id}/nodes/{node_id}.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeid.html
func (c *v2Client) GetNode(ctx context.Context, projectID, nodeID string) (*Node, error) {
	var out Node
	if err := c.do(ctx, http.MethodGet, nodePath(projectID, nodeID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateNode is POST /v2/projects/{project_id}/nodes (template-less node).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
func (c *v2Client) CreateNode(ctx context.Context, projectID string, req NodeCreateRequest) (*Node, error) {
	var out Node
	if err := c.do(ctx, http.MethodPost, nodesPath(projectID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateNodeFromTemplate is POST /v2/projects/{project_id}/templates/{template_id}.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/projectsprojectidtemplatestemplateid.html
func (c *v2Client) CreateNodeFromTemplate(ctx context.Context, projectID, templateID string, req NodeFromTemplateRequest) (*Node, error) {
	var out Node
	endpoint := "/v2/projects/" + pathEscape(projectID) + "/templates/" + pathEscape(templateID)
	if err := c.do(ctx, http.MethodPost, endpoint, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateNode is PUT /v2/projects/{project_id}/nodes/{node_id}.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeid.html
func (c *v2Client) UpdateNode(ctx context.Context, projectID, nodeID string, req NodeUpdateRequest) (*Node, error) {
	var out Node
	if err := c.do(ctx, http.MethodPut, nodePath(projectID, nodeID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteNode is DELETE /v2/projects/{project_id}/nodes/{node_id} (204).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeid.html
func (c *v2Client) DeleteNode(ctx context.Context, projectID, nodeID string) error {
	return c.do(ctx, http.MethodDelete, nodePath(projectID, nodeID), nil, nil)
}

// StartNode is POST /v2/projects/{project_id}/nodes/{node_id}/start.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeidstart.html
func (c *v2Client) StartNode(ctx context.Context, projectID, nodeID string) (*Node, error) {
	var out Node
	if err := c.do(ctx, http.MethodPost, nodePath(projectID, nodeID)+"/start", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// StopNode is POST /v2/projects/{project_id}/nodes/{node_id}/stop.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeidstop.html
func (c *v2Client) StopNode(ctx context.Context, projectID, nodeID string) (*Node, error) {
	var out Node
	if err := c.do(ctx, http.MethodPost, nodePath(projectID, nodeID)+"/stop", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// nodeFilePath escapes each segment of a path relative to the node directory.
func nodeFilePath(projectID, nodeID, path string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		segs[i] = pathEscape(s)
	}
	return nodePath(projectID, nodeID) + "/files/" + strings.Join(segs, "/")
}

// ReadNodeFile is GET /v2/projects/{project_id}/nodes/{node_id}/files/{path}.
// The docs list only 204; 2.2.61 answers 200 with the raw file content, and
// 404 when the file does not exist (docs/api-drift.md).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeidfilespath.html
func (c *v2Client) ReadNodeFile(ctx context.Context, projectID, nodeID, path string) ([]byte, error) {
	body, _, err := c.send(ctx, http.MethodGet, nodeFilePath(projectID, nodeID, path), nil, "")
	return body, err
}

// WriteNodeFile is POST /v2/projects/{project_id}/nodes/{node_id}/files/{path}
// with the raw file content as the body.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeidfilespath.html
func (c *v2Client) WriteNodeFile(ctx context.Context, projectID, nodeID, path string, content []byte) error {
	if content == nil {
		content = []byte{}
	}
	_, _, err := c.send(ctx, http.MethodPost, nodeFilePath(projectID, nodeID, path), content, "application/octet-stream")
	return err
}
