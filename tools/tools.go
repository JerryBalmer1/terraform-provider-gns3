//go:build tools

// Package tools pins build-time tools in go.mod. Run them with
// `go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs`.
package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
