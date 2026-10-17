package gns3

import (
	"context"
	"encoding/json"
	"strings"
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
	if !equalTemplates(vpcs, want) {
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
	// Type-specific keys sit at the top level of a user template.
	if string(tpl.Properties["base_script_file"]) != `"vpcs_base_config.txt"` || string(tpl.Properties["console_type"]) != `"telnet"` {
		t.Fatalf("properties = %v", tpl.Properties)
	}
	if _, ok := tpl.Properties["name"]; ok {
		t.Fatal("common field name ended up in properties")
	}
}

func TestGetTemplateBuiltinProperties(t *testing.T) {
	tpl, err := replay(t, "templates/get_builtin").GetTemplate(context.Background(), "19021f99-e36f-394d-b4a1-8aaa902ab9cc")
	if err != nil {
		t.Fatal(err)
	}
	// A built-in keeps them under "properties"; both shapes decode the same way.
	if !tpl.Builtin || tpl.ComputeID != nil || string(tpl.Properties["base_script_file"]) != `"vpcs_base_config.txt"` {
		t.Fatalf("got %+v", tpl)
	}
	if _, ok := tpl.Properties["properties"]; ok {
		t.Fatal("nested properties object was not flattened")
	}
}

func TestGetTemplateNotFound(t *testing.T) {
	_, err := replay(t, "templates/get_404").GetTemplate(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want 404", err)
	}
}

func TestCreateTemplate(t *testing.T) {
	tpl, err := replay(t, "templates/create").CreateTemplate(context.Background(), TemplateCreateRequest{
		Name: "tf-fixture-vpcs", TemplateType: "vpcs", ComputeID: "local",
		Properties: map[string]json.RawMessage{"console_type": json.RawMessage(`"none"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tpl.TemplateID != "5bb744cc-7ef9-49c0-a50a-c383d00bf3d5" || tpl.Category != "guest" || tpl.DefaultNameFormat != "PC{0}" ||
		tpl.Symbol != ":/symbols/vpcs_guest.svg" || tpl.Builtin {
		t.Fatalf("got %+v", tpl)
	}
	if string(tpl.Properties["console_type"]) != `"none"` {
		t.Fatalf("properties = %v", tpl.Properties)
	}
}

func TestCreateTemplateRejected(t *testing.T) {
	_, err := replay(t, "templates/create_400").CreateTemplate(context.Background(), TemplateCreateRequest{
		Name: "tf-fixture-bad", TemplateType: "vpcs", ComputeID: "local",
		Properties: map[string]json.RawMessage{"ram": json.RawMessage(`256`)},
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "'ram' was unexpected") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateTemplateRefusesCommonKeyInProperties(t *testing.T) {
	_, err := NewV2(Config{Host: "127.0.0.1", Port: 1}).CreateTemplate(context.Background(), TemplateCreateRequest{
		Name: "x", TemplateType: "vpcs", ComputeID: "local",
		Properties: map[string]json.RawMessage{"name": json.RawMessage(`"y"`)},
	})
	if err == nil || !strings.Contains(err.Error(), `"name" is a common template field`) {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateTemplate(t *testing.T) {
	tpl, err := replay(t, "templates/update").UpdateTemplate(context.Background(), "5bb744cc-7ef9-49c0-a50a-c383d00bf3d5", TemplateUpdateRequest{
		Name: "tf-fixture-vpcs-renamed", Properties: map[string]json.RawMessage{"console_type": json.RawMessage(`"telnet"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Name != "tf-fixture-vpcs-renamed" || string(tpl.Properties["console_type"]) != `"telnet"` {
		t.Fatalf("got %+v", tpl)
	}
}

func TestDeleteTemplate(t *testing.T) {
	if err := replay(t, "templates/delete").DeleteTemplate(context.Background(), "5bb744cc-7ef9-49c0-a50a-c383d00bf3d5"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteBuiltinTemplate(t *testing.T) {
	err := replay(t, "templates/delete_builtin_409").DeleteTemplate(context.Background(), "19021f99-e36f-394d-b4a1-8aaa902ab9cc")
	if err == nil || !strings.Contains(err.Error(), "HTTP 409") || !strings.Contains(err.Error(), "because it is a builtin") {
		t.Fatalf("err = %v", err)
	}
}

func equalTemplates(a, b Template) bool {
	return a.TemplateID == b.TemplateID && a.Name == b.Name && a.TemplateType == b.TemplateType &&
		a.Category == b.Category && a.Symbol == b.Symbol && a.Builtin == b.Builtin &&
		a.DefaultNameFormat == b.DefaultNameFormat && (a.ComputeID == nil) == (b.ComputeID == nil)
}
