package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/provider"
)

// Actions, list resources and functions are optional provider capabilities.
// Each one's method lives in its own file: Actions here, ListResources in
// list_provider.go, Functions in functions_provider.go.
var (
	_ provider.ProviderWithActions       = (*gns3Provider)(nil)
	_ provider.ProviderWithListResources = (*gns3Provider)(nil)
	_ provider.ProviderWithFunctions     = (*gns3Provider)(nil)
)

// Actions returns the provider's actions. Terraform 1.14 and later invoke
// them; older Terraform ignores them, so configurations without an action
// block keep working there.
func (p *gns3Provider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		NewStartNodesAction,
		NewStopNodesAction,
		NewSuspendNodesAction,
		NewReloadNodesAction,
		NewSnapshotAction,
	}
}

// sendProgress reports one progress event. The framework always sets
// SendProgress during Invoke; the nil check keeps direct calls in tests safe.
func sendProgress(resp *action.InvokeResponse, msg string) {
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: msg})
	}
}
