package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/list"
)

// ListResources are the list resources for terraform query (Terraform 1.14
// and later). Each shares its type name, and its struct, with the managed
// resource it lists.
func (p *gns3Provider) ListResources(_ context.Context) []func() list.ListResource {
	return []func() list.ListResource{
		NewProjectListResource,
		NewNodeListResource,
		NewTemplateListResource,
	}
}

// listLimitReached reports whether n results satisfy limit (0 means no limit).
func listLimitReached(n int, limit int64) bool {
	return limit > 0 && int64(n) >= limit
}
