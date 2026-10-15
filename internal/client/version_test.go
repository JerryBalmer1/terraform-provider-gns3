package client

import (
	"context"
	"testing"
)

func TestGetVersion(t *testing.T) {
	v, err := replay(t, "version/get").GetVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *v != (Version{Version: "2.2.61", Local: true}) {
		t.Fatalf("got %+v", v)
	}
}
