package gns3

import (
	"context"
	"testing"
)

var localCompute = Compute{
	ComputeID: "local", Name: "MSI", Host: "127.0.0.1", Port: 3080, Protocol: "http", Connected: true,
}

func TestListComputes(t *testing.T) {
	got, err := replay(t, "computes/list").ListComputes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != localCompute {
		t.Fatalf("got %+v", got)
	}
}

func TestGetCompute(t *testing.T) {
	got, err := replay(t, "computes/get").GetCompute(context.Background(), "local")
	if err != nil {
		t.Fatal(err)
	}
	if *got != localCompute {
		t.Fatalf("got %+v", got)
	}
}
