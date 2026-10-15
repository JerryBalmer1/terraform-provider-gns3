package client

import (
	"context"
	"net/http"
)

// TemplatesAPI covers /v2/templates (read-only in 0.1.0).
type TemplatesAPI interface {
	ListTemplates(ctx context.Context) ([]Template, error)
	GetTemplate(ctx context.Context, templateID string) (*Template, error)
}

// Template is the common part of every template object; type-specific fields
// (ram, adapters, ...) are not decoded.
type Template struct {
	TemplateID        string  `json:"template_id"`
	Name              string  `json:"name"`
	TemplateType      string  `json:"template_type"`
	ComputeID         *string `json:"compute_id"` // null for built-in templates
	Category          string  `json:"category"`
	Symbol            string  `json:"symbol"`
	Builtin           bool    `json:"builtin"`
	DefaultNameFormat string  `json:"default_name_format"`
}

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
	if err := c.do(ctx, http.MethodGet, "/v2/templates/"+pathEscape(templateID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
