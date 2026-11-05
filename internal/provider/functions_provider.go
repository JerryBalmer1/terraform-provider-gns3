package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

// Functions returns the provider-defined functions. They are pure: none of
// them uses the GNS3 client, so they work without a reachable server.
func (p *gns3Provider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{
		newConsoleAddressFunction,
		newNodeNameFunction,
	}
}
