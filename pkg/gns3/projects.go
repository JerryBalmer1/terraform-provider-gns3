package gns3

import (
	"context"
	"net/http"
)

// ProjectsAPI covers /v2/projects.
type ProjectsAPI interface {
	ListProjects(ctx context.Context) ([]Project, error)
	GetProject(ctx context.Context, projectID string) (*Project, error)
	CreateProject(ctx context.Context, req ProjectCreateRequest) (*Project, error)
	UpdateProject(ctx context.Context, projectID string, req ProjectUpdateRequest) (*Project, error)
	OpenProject(ctx context.Context, projectID string) (*Project, error)
	CloseProject(ctx context.Context, projectID string) error
	DeleteProject(ctx context.Context, projectID string) error
}

// Project is the subset of the project object the provider manages.
type Project struct {
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	Status      string `json:"status"` // "opened" or "closed"
	AutoStart   bool   `json:"auto_start"`
	AutoOpen    bool   `json:"auto_open"`
	AutoClose   bool   `json:"auto_close"`
	SceneWidth  int64  `json:"scene_width"`
	SceneHeight int64  `json:"scene_height"`
	GridSize    int64  `json:"grid_size"`
	ShowGrid    bool   `json:"show_grid"`
}

// ProjectCreateRequest is the POST /v2/projects body. Nil fields are omitted
// so the server applies its own defaults.
type ProjectCreateRequest struct {
	Name        string  `json:"name"`
	Path        *string `json:"path,omitempty"`
	AutoStart   *bool   `json:"auto_start,omitempty"`
	AutoOpen    *bool   `json:"auto_open,omitempty"`
	AutoClose   *bool   `json:"auto_close,omitempty"`
	SceneWidth  *int64  `json:"scene_width,omitempty"`
	SceneHeight *int64  `json:"scene_height,omitempty"`
	GridSize    *int64  `json:"grid_size,omitempty"`
	ShowGrid    *bool   `json:"show_grid,omitempty"`
}

// ProjectUpdateRequest is the PUT /v2/projects/{project_id} body.
type ProjectUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	AutoStart   *bool   `json:"auto_start,omitempty"`
	AutoOpen    *bool   `json:"auto_open,omitempty"`
	AutoClose   *bool   `json:"auto_close,omitempty"`
	SceneWidth  *int64  `json:"scene_width,omitempty"`
	SceneHeight *int64  `json:"scene_height,omitempty"`
	GridSize    *int64  `json:"grid_size,omitempty"`
	ShowGrid    *bool   `json:"show_grid,omitempty"`
}

// ListProjects is GET /v2/projects.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projects.html
func (c *v2Client) ListProjects(ctx context.Context) ([]Project, error) {
	var out []Project
	if err := c.do(ctx, http.MethodGet, "/v2/projects", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetProject is GET /v2/projects/{project_id}.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectid.html
func (c *v2Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	var out Project
	if err := c.do(ctx, http.MethodGet, "/v2/projects/"+pathEscape(projectID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateProject is POST /v2/projects. The server returns the project opened.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projects.html
func (c *v2Client) CreateProject(ctx context.Context, req ProjectCreateRequest) (*Project, error) {
	var out Project
	if err := c.do(ctx, http.MethodPost, "/v2/projects", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateProject is PUT /v2/projects/{project_id}.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectid.html
func (c *v2Client) UpdateProject(ctx context.Context, projectID string, req ProjectUpdateRequest) (*Project, error) {
	var out Project
	if err := c.do(ctx, http.MethodPut, "/v2/projects/"+pathEscape(projectID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OpenProject is POST /v2/projects/{project_id}/open (201 with the project).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectidopen.html
func (c *v2Client) OpenProject(ctx context.Context, projectID string) (*Project, error) {
	var out Project
	if err := c.do(ctx, http.MethodPost, "/v2/projects/"+pathEscape(projectID)+"/open", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CloseProject is POST /v2/projects/{project_id}/close (204).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectidclose.html
func (c *v2Client) CloseProject(ctx context.Context, projectID string) error {
	return c.do(ctx, http.MethodPost, "/v2/projects/"+pathEscape(projectID)+"/close", struct{}{}, nil)
}

// DeleteProject is DELETE /v2/projects/{project_id} (204).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectid.html
func (c *v2Client) DeleteProject(ctx context.Context, projectID string) error {
	return c.do(ctx, http.MethodDelete, "/v2/projects/"+pathEscape(projectID), nil, nil)
}
