package provider

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

var (
	_ function.Function = (*consoleAddressFunction)(nil)
	_ function.Function = (*nodeNameFunction)(nil)
)

// consoleAddressFunction implements provider::gns3::console_address.
type consoleAddressFunction struct{}

func newConsoleAddressFunction() function.Function { return &consoleAddressFunction{} }

func (f *consoleAddressFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "console_address"
}

func (f *consoleAddressFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Joins a console host and port into host:port",
		MarkdownDescription: "Joins a console host and a console port into `host:port`, for example " +
			"`console_address(\"127.0.0.1\", gns3_node.r1.console)` gives `127.0.0.1:5000`. " +
			"An IPv6 host is bracketed (`[::1]:5000`). The host must not be empty and the port must be " +
			"between 1 and 65535.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "host",
				MarkdownDescription: "Host name or IP address of the console, usually the GNS3 server host. Must not be empty.",
			},
			function.Int64Parameter{
				Name:                "console",
				MarkdownDescription: "Console TCP port, such as a node's `console`. Must be between 1 and 65535.",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *consoleAddressFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var host string
	var console int64
	resp.Error = req.Arguments.Get(ctx, &host, &console)
	if resp.Error != nil {
		return
	}
	if host == "" {
		resp.Error = function.NewArgumentFuncError(0, "host must not be empty")
		return
	}
	if console < 1 || console > 65535 {
		resp.Error = function.NewArgumentFuncError(1, fmt.Sprintf("console must be between 1 and 65535, got %d", console))
		return
	}
	resp.Error = resp.Result.Set(ctx, net.JoinHostPort(host, strconv.FormatInt(console, 10)))
}

// nodeNameFunction implements provider::gns3::node_name.
type nodeNameFunction struct{}

func newNodeNameFunction() function.Function { return &nodeNameFunction{} }

func (f *nodeNameFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "node_name"
}

func (f *nodeNameFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Builds a node name from a prefix and a zero-padded index",
		MarkdownDescription: "Builds a node name as `<prefix>-<index>`, with the index zero-padded to at " +
			"least two digits: `node_name(\"r\", 1)` gives `r-01`, `node_name(\"r\", 12)` gives `r-12` and " +
			"`node_name(\"r\", 100)` gives `r-100`. Useful with `count.index` or `for_each`. The prefix must " +
			"not be empty and the index must not be negative.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "prefix",
				MarkdownDescription: "Text before the dash, such as `r` or `sw`. Must not be empty.",
			},
			function.Int64Parameter{
				Name:                "index",
				MarkdownDescription: "Number after the dash, zero-padded to at least two digits. Must be 0 or more.",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *nodeNameFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var prefix string
	var index int64
	resp.Error = req.Arguments.Get(ctx, &prefix, &index)
	if resp.Error != nil {
		return
	}
	if prefix == "" {
		resp.Error = function.NewArgumentFuncError(0, "prefix must not be empty")
		return
	}
	if index < 0 {
		resp.Error = function.NewArgumentFuncError(1, fmt.Sprintf("index must not be negative, got %d", index))
		return
	}
	resp.Error = resp.Result.Set(ctx, fmt.Sprintf("%s-%02d", prefix, index))
}
