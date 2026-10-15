package client

import (
	"context"
	"testing"
)

func TestListProjects(t *testing.T) {
	got, err := replay(t, "projects/list").ListProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ProjectID != "1af687e8-af70-40b8-87a9-ee9625c9360b" || got[0].Name != "test" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetProject(t *testing.T) {
	p, err := replay(t, "projects/get").GetProject(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b")
	if err != nil {
		t.Fatal(err)
	}
	want := Project{
		ProjectID: "1af687e8-af70-40b8-87a9-ee9625c9360b", Name: "test",
		Path: `C:\Users\jlbal\AppData\Local\Temp\gns3\data\projects\1af687e8-af70-40b8-87a9-ee9625c9360b`, Filename: "test.gns3",
		Status: "opened", AutoClose: true, SceneWidth: 2000, SceneHeight: 1000, GridSize: 75,
	}
	if *p != want {
		t.Fatalf("got  %+v\nwant %+v", *p, want)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	_, err := replay(t, "projects/get_404").GetProject(context.Background(), "ffffffff-0000-0000-0000-000000000000")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want 404", err)
	}
	want := "GNS3 API GET /v2/projects/ffffffff-0000-0000-0000-000000000000 returned HTTP 404: Project ID ffffffff-0000-0000-0000-000000000000 doesn't exist"
	if err.Error() != want {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestCreateProject(t *testing.T) {
	p, err := replay(t, "projects/create").CreateProject(context.Background(), ProjectCreateRequest{
		Name: "test", AutoClose: ptr(true), AutoOpen: ptr(false), AutoStart: ptr(false),
		SceneWidth: ptr[int64](2000), SceneHeight: ptr[int64](1000), GridSize: ptr[int64](75), ShowGrid: ptr(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ProjectID != "1af687e8-af70-40b8-87a9-ee9625c9360b" || p.Status != "opened" || p.Filename != "test.gns3" {
		t.Fatalf("got %+v", p)
	}
}

func TestUpdateProject(t *testing.T) {
	p, err := replay(t, "projects/update").UpdateProject(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b",
		ProjectUpdateRequest{Name: ptr("test2")})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "test2" {
		t.Fatalf("got %+v", p)
	}
}

func TestOpenProject(t *testing.T) {
	p, err := replay(t, "projects/open").OpenProject(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != "opened" {
		t.Fatalf("status = %q", p.Status)
	}
}

func TestCloseProject(t *testing.T) {
	if err := replay(t, "projects/close").CloseProject(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteProject(t *testing.T) {
	if err := replay(t, "projects/delete").DeleteProject(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b"); err != nil {
		t.Fatal(err)
	}
}
