package gns3

import (
	"context"
	"testing"
)

func TestListTemplates(t *testing.T) {
	got, err := replay(t, "templates/list").ListTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 7 {
		t.Fatalf("got %d templates", len(got))
	}
	var vpcs Template
	for _, tpl := range got {
		if tpl.Name == "VPCS" {
			vpcs = tpl
		}
	}
	want := Template{
		TemplateID: "19021f99-e36f-394d-b4a1-8aaa902ab9cc", Name: "VPCS", TemplateType: "vpcs",
		Category: "guest", Symbol: ":/symbols/vpcs_guest.svg", Builtin: true, DefaultNameFormat: "PC{0}",
	}
	if vpcs != want {
		t.Fatalf("got  %+v\nwant %+v", vpcs, want)
	}
}

func TestGetTemplate(t *testing.T) {
	tpl, err := replay(t, "templates/get").GetTemplate(context.Background(), "e1fd1ccb-8d9d-4e27-a4c4-f26edaba5336")
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Name != "VPCS_TEST" || tpl.ComputeID == nil || *tpl.ComputeID != "local" || tpl.Builtin {
		t.Fatalf("got %+v", tpl)
	}
}

func TestGetTemplateNotFound(t *testing.T) {
	_, err := replay(t, "templates/get_404").GetTemplate(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want 404", err)
	}
}
