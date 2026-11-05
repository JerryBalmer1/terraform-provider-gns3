package gns3

import (
	"context"
	"net/http"
)

// SnapshotsAPI covers /v2/projects/{project_id}/snapshots.
type SnapshotsAPI interface {
	ListSnapshots(ctx context.Context, projectID string) ([]Snapshot, error)
	CreateSnapshot(ctx context.Context, projectID, name string) (*Snapshot, error)
	RestoreSnapshot(ctx context.Context, projectID, snapshotID string) error
	DeleteSnapshot(ctx context.Context, projectID, snapshotID string) error
}

// Snapshot is the snapshot object. CreatedAt is a UTC Unix timestamp in
// seconds.
type Snapshot struct {
	SnapshotID string `json:"snapshot_id"`
	ProjectID  string `json:"project_id"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"created_at"`
}

func snapshotsPath(projectID string) string {
	return "/v2/projects/" + pathEscape(projectID) + "/snapshots"
}

func snapshotPath(projectID, snapshotID string) string {
	return snapshotsPath(projectID) + "/" + pathEscape(snapshotID)
}

// ListSnapshots is GET /v2/projects/{project_id}/snapshots (200).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/snapshot/projectsprojectidsnapshots.html
func (c *v2Client) ListSnapshots(ctx context.Context, projectID string) ([]Snapshot, error) {
	var out []Snapshot
	if err := c.do(ctx, http.MethodGet, snapshotsPath(projectID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSnapshot is POST /v2/projects/{project_id}/snapshots with {"name": ...}
// (201, the snapshot).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/snapshot/projectsprojectidsnapshots.html
func (c *v2Client) CreateSnapshot(ctx context.Context, projectID, name string) (*Snapshot, error) {
	var out Snapshot
	req := struct {
		Name string `json:"name"`
	}{name}
	if err := c.do(ctx, http.MethodPost, snapshotsPath(projectID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RestoreSnapshot is POST /v2/projects/{project_id}/snapshots/{snapshot_id}/restore
// (201, the project). The project body is not decoded; callers re-read what
// they need.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/snapshot/projectsprojectidsnapshotssnapshotidrestore.html
func (c *v2Client) RestoreSnapshot(ctx context.Context, projectID, snapshotID string) error {
	return c.do(ctx, http.MethodPost, snapshotPath(projectID, snapshotID)+"/restore", struct{}{}, nil)
}

// DeleteSnapshot is DELETE /v2/projects/{project_id}/snapshots/{snapshot_id} (204).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/snapshot/projectsprojectidsnapshotssnapshotid.html
func (c *v2Client) DeleteSnapshot(ctx context.Context, projectID, snapshotID string) error {
	return c.do(ctx, http.MethodDelete, snapshotPath(projectID, snapshotID), nil, nil)
}
