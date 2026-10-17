package gns3

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
)

// TemplatesAPI covers /v2/templates.
type TemplatesAPI interface {
	ListTemplates(ctx context.Context) ([]Template, error)
	GetTemplate(ctx context.Context, templateID string) (*Template, error)
	CreateTemplate(ctx context.Context, req TemplateCreateRequest) (*Template, error)
	UpdateTemplate(ctx context.Context, templateID string, req TemplateUpdateRequest) (*Template, error)
	DeleteTemplate(ctx context.Context, templateID string) error
}

// Template is a template object. The common fields are decoded into the
// struct; every other key (ram, adapters, image, ...) goes into Properties.
// User templates carry those keys at the top level, built-in templates under
// "properties" (seen on 2.2.61, docs/api-drift.md); both end up in Properties.
type Template struct {
	TemplateID        string  `json:"template_id"`
	Name              string  `json:"name"`
	TemplateType      string  `json:"template_type"`
	ComputeID         *string `json:"compute_id"` // null for built-in templates
	Category          string  `json:"category"`
	Symbol            string  `json:"symbol"`
	Builtin           bool    `json:"builtin"`
	DefaultNameFormat string  `json:"default_name_format"`

	Properties map[string]json.RawMessage `json:"-"`
}

// TemplateCommonKeys are the template keys that are not type-specific: the
// fields of the base template schema on
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/templates.html.
var TemplateCommonKeys = []string{
	"template_id", "name", "template_type", "compute_id", "category", "symbol", "builtin", "default_name_format",
}

// UnmarshalJSON decodes the common fields and collects the rest in Properties.
func (t *Template) UnmarshalJSON(b []byte) error {
	type common Template
	if err := json.Unmarshal(b, (*common)(t)); err != nil {
		return err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return err
	}
	for _, k := range TemplateCommonKeys {
		delete(all, k)
	}
	if nested, ok := all["properties"]; ok {
		var inner map[string]json.RawMessage
		if json.Unmarshal(nested, &inner) == nil {
			delete(all, "properties")
			maps.Copy(all, inner)
		}
	}
	t.Properties = all
	return nil
}

// TemplateCreateRequest is the POST /v2/templates body. Properties are sent
// at the top level next to the common fields, as in the docs sample.
type TemplateCreateRequest struct {
	Name         string
	TemplateType string
	ComputeID    string
	Properties   map[string]json.RawMessage
}

// MarshalJSON flattens Properties into the body.
func (r TemplateCreateRequest) MarshalJSON() ([]byte, error) {
	body, err := flatten(r.Properties)
	if err != nil {
		return nil, err
	}
	body["name"], _ = json.Marshal(r.Name)
	body["template_type"], _ = json.Marshal(r.TemplateType)
	body["compute_id"], _ = json.Marshal(r.ComputeID)
	return json.Marshal(body)
}

// TemplateUpdateRequest is the PUT /v2/templates/{template_id} body. 2.2.61
// merges it into the stored template and ignores template_type, compute_id,
// template_id and builtin, so those are not sent.
type TemplateUpdateRequest struct {
	Name       string
	Properties map[string]json.RawMessage
}

// MarshalJSON flattens Properties into the body.
func (r TemplateUpdateRequest) MarshalJSON() ([]byte, error) {
	body, err := flatten(r.Properties)
	if err != nil {
		return nil, err
	}
	body["name"], _ = json.Marshal(r.Name)
	return json.Marshal(body)
}

// flatten copies props, refusing the common keys so a property can't
// silently replace a field set elsewhere.
func flatten(props map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	body := make(map[string]json.RawMessage, len(props)+3)
	for k, v := range props {
		if slices.Contains(TemplateCommonKeys, k) {
			return nil, fmt.Errorf("template property %q is a common template field, not a type-specific property", k)
		}
		body[k] = v
	}
	return body, nil
}

func templatePath(templateID string) string { return "/v2/templates/" + pathEscape(templateID) }

// ListTemplates is GET /v2/templates.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/templates.html
func (c *v2Client) ListTemplates(ctx context.Context) ([]Template, error) {
	var out []Template
	if err := c.do(ctx, http.MethodGet, "/v2/templates", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTemplate is GET /v2/templates/{template_id}.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/templatestemplateid.html
func (c *v2Client) GetTemplate(ctx context.Context, templateID string) (*Template, error) {
	var out Template
	if err := c.do(ctx, http.MethodGet, templatePath(templateID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateTemplate is POST /v2/templates. 2.2.61 checks the body against the
// type's own schema here (unknown keys and wrong types are 400) and fills in
// every default.
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/templates.html
func (c *v2Client) CreateTemplate(ctx context.Context, req TemplateCreateRequest) (*Template, error) {
	var out Template
	if err := c.do(ctx, http.MethodPost, "/v2/templates", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTemplate is PUT /v2/templates/{template_id}. 2.2.61 checks only the
// common fields here and stores any type-specific key as given
// (docs/api-drift.md).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/templatestemplateid.html
func (c *v2Client) UpdateTemplate(ctx context.Context, templateID string, req TemplateUpdateRequest) (*Template, error) {
	var out Template
	if err := c.do(ctx, http.MethodPut, templatePath(templateID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTemplate is DELETE /v2/templates/{template_id}. A built-in template
// answers 409 on 2.2.61 (docs/api-drift.md).
// https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/templatestemplateid.html
func (c *v2Client) DeleteTemplate(ctx context.Context, templateID string) error {
	return c.do(ctx, http.MethodDelete, templatePath(templateID), nil, nil)
}
