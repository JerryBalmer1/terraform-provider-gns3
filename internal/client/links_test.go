package client

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestListLinks(t *testing.T) {
	got, err := replay(t, "links/list").ListLinks(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LinkID != "3013a84e-8c9a-48e1-841c-a6448ec3c1ca" || len(got[0].Nodes) != 2 {
		t.Fatalf("got %+v", got)
	}
	if !jsonEqual(t, got[0].Filters, []byte(`{"frequency_drop":[50],"latency":[10]}`)) {
		t.Errorf("filters = %s", got[0].Filters)
	}
}

func TestGetLink(t *testing.T) {
	l, err := replay(t, "links/get").GetLink(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "3013a84e-8c9a-48e1-841c-a6448ec3c1ca")
	if err != nil {
		t.Fatal(err)
	}
	want := []LinkNode{
		{NodeID: "0000895b-c1fc-49a7-90b6-60b6e998c3d0", AdapterNumber: 0, PortNumber: 0},
		{NodeID: "115dbaac-1970-489a-a720-7e5085118c2d", AdapterNumber: 0, PortNumber: 0},
	}
	if !reflect.DeepEqual(l.Nodes, want) || l.LinkType != "ethernet" || l.Suspend {
		t.Fatalf("got %+v", l)
	}
	if string(l.Filters) != "{}" {
		t.Errorf("filters = %s", l.Filters)
	}
}

func TestCreateLink(t *testing.T) {
	l, err := replay(t, "links/create").CreateLink(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b", LinkCreateRequest{
		Nodes: []LinkNode{
			{NodeID: "0000895b-c1fc-49a7-90b6-60b6e998c3d0"},
			{NodeID: "115dbaac-1970-489a-a720-7e5085118c2d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if l.LinkID != "3013a84e-8c9a-48e1-841c-a6448ec3c1ca" {
		t.Fatalf("got %+v", l)
	}
}

func TestCreateLinkConflict(t *testing.T) {
	_, err := replay(t, "links/create_409").CreateLink(context.Background(), "1af687e8-af70-40b8-87a9-ee9625c9360b", LinkCreateRequest{
		Nodes: []LinkNode{
			{NodeID: "526c2a0d-1a9e-463b-bb69-d48083d42db2", PortNumber: 3},
			{NodeID: "526c2a0d-1a9e-463b-bb69-d48083d42db2", PortNumber: 4},
		},
	})
	want := "GNS3 API POST /v2/projects/1af687e8-af70-40b8-87a9-ee9625c9360b/links returned HTTP 409: Cannot connect to itself"
	if err == nil || strings.SplitN(err.Error(), "\n", 2)[0] != want || !strings.Contains(err.Error(), "\nRequest body: {\"nodes\":") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateLink(t *testing.T) {
	l, err := replay(t, "links/update").UpdateLink(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "3013a84e-8c9a-48e1-841c-a6448ec3c1ca",
		LinkUpdateRequest{Suspend: ptr(true), Filters: json.RawMessage(`{"frequency_drop":[50],"latency":[10]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if !l.Suspend {
		t.Fatalf("got %+v", l)
	}
}

func TestDeleteLink(t *testing.T) {
	err := replay(t, "links/delete").DeleteLink(context.Background(),
		"1af687e8-af70-40b8-87a9-ee9625c9360b", "3013a84e-8c9a-48e1-841c-a6448ec3c1ca")
	if err != nil {
		t.Fatal(err)
	}
}
