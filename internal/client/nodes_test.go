package client

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

var vpcsPorts = []Port{{AdapterNumber: 0, PortNumber: 0, Name: "Ethernet0", ShortName: "e0", LinkType: "ethernet"}}

func TestListNodes(t *testing.T) {
	got, err := replay(t, "nodes/list").ListNodes(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].NodeID != "0000895b-c1fc-49a7-90b6-60b6e998c3d0" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetNode(t *testing.T) {
	n, err := replay(t, "nodes/get").GetNode(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "0000895b-c1fc-49a7-90b6-60b6e998c3d0")
	if err != nil {
		t.Fatal(err)
	}
	if n.Name != "test" || n.NodeType != "vpcs" || n.ComputeID != "local" || n.Status != "stopped" {
		t.Errorf("got %+v", n)
	}
	if n.TemplateID != nil {
		t.Errorf("template_id = %q, want null", *n.TemplateID)
	}
	if n.Console == nil || *n.Console != 5000 || n.ConsoleType == nil || *n.ConsoleType != "telnet" {
		t.Errorf("console = %v / %v", n.Console, n.ConsoleType)
	}
	if n.Symbol == nil || *n.Symbol != ":/symbols/computer.svg" || n.Z != 1 {
		t.Errorf("symbol/z = %v / %d", n.Symbol, n.Z)
	}
	if !reflect.DeepEqual(n.Ports, vpcsPorts) {
		t.Errorf("ports = %+v", n.Ports)
	}
	if string(n.Properties["startup_script"]) != `"echo test"` {
		t.Errorf("properties = %v", n.Properties)
	}
}

func TestGetNodeNotFound(t *testing.T) {
	_, err := replay(t, "nodes/get_404").GetNode(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "00000000-0000-0000-0000-000000000000")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want 404", err)
	}
}

func TestCreateNode(t *testing.T) {
	n, err := replay(t, "nodes/create").CreateNode(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b", NodeCreateRequest{
		ComputeID: "local", Name: "test", NodeType: "vpcs",
		Properties: map[string]json.RawMessage{"startup_script": json.RawMessage(`"echo test"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n.NodeID != "0000895b-c1fc-49a7-90b6-60b6e998c3d0" || !reflect.DeepEqual(n.Ports, vpcsPorts) {
		t.Fatalf("got %+v", n)
	}
}

func TestCreateNodeFromTemplate(t *testing.T) {
	n, err := replay(t, "nodes/create_from_template").CreateNodeFromTemplate(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "19021f99-e36f-394d-b4a1-8aaa902ab9cc",
		NodeFromTemplateRequest{X: 100, Y: 200, Name: ptr("PC1"), ComputeID: ptr("local")})
	if err != nil {
		t.Fatal(err)
	}
	if n.TemplateID == nil || *n.TemplateID != "19021f99-e36f-394d-b4a1-8aaa902ab9cc" || n.X != 100 || n.Y != 200 {
		t.Fatalf("got %+v", n)
	}
}

func TestUpdateNode(t *testing.T) {
	n, err := replay(t, "nodes/update").UpdateNode(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "115dbaac-1970-489a-a720-7e5085118c2d",
		NodeUpdateRequest{ComputeID: "local", Name: "PC1", NodeType: "vpcs", X: ptr[int64](10), Y: ptr[int64](20), Z: ptr[int64](1)})
	if err != nil {
		t.Fatal(err)
	}
	if n.X != 10 || n.Y != 20 {
		t.Fatalf("got %+v", n)
	}
}

func TestDeleteNode(t *testing.T) {
	err := replay(t, "nodes/delete").DeleteNode(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "0000895b-c1fc-49a7-90b6-60b6e998c3d0")
	if err != nil {
		t.Fatal(err)
	}
}

func TestStartNode(t *testing.T) {
	n, err := replay(t, "nodes/start").StartNode(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "115dbaac-1970-489a-a720-7e5085118c2d")
	if err != nil {
		t.Fatal(err)
	}
	// 2.2.61 answers with the node as it was before the start ("stopped");
	// a GET right after reports "started" (docs/api-drift.md). Callers re-read.
	if n.NodeID != "115dbaac-1970-489a-a720-7e5085118c2d" || n.Status != "stopped" {
		t.Fatalf("got %+v", n)
	}
}

func TestWriteNodeFile(t *testing.T) {
	err := replay(t, "nodes/file_write").WriteNodeFile(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "115dbaac-1970-489a-a720-7e5085118c2d", "startup.vpc", []byte("set pcname PC1"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadNodeFile(t *testing.T) {
	got, err := replay(t, "nodes/file_read").ReadNodeFile(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "115dbaac-1970-489a-a720-7e5085118c2d", "startup.vpc")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "set pcname PC1" {
		t.Fatalf("got %q", got)
	}
}

func TestReadNodeFileNotFound(t *testing.T) {
	_, err := replay(t, "nodes/file_read_404").ReadNodeFile(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "115dbaac-1970-489a-a720-7e5085118c2d", "missing.txt")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want 404", err)
	}
}

func TestStopNode(t *testing.T) {
	n, err := replay(t, "nodes/stop").StopNode(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "115dbaac-1970-489a-a720-7e5085118c2d")
	if err != nil {
		t.Fatal(err)
	}
	if n.Status != "stopped" {
		t.Fatalf("status = %q", n.Status)
	}
}
