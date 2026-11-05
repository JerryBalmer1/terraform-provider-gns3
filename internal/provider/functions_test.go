package provider

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

type funcCase struct {
	name    string
	args    []attr.Value
	want    string
	wantErr string // substring of the error text; empty means success
	wantArg int64  // argument position the error names
}

func runFunctionCases(t *testing.T, f function.Function, cases []funcCase) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := function.RunResponse{Result: function.NewResultData(types.StringUnknown())}
			f.Run(ctx, function.RunRequest{Arguments: function.NewArgumentsData(tc.args)}, &resp)
			if tc.wantErr != "" {
				if resp.Error == nil {
					t.Fatalf("want error containing %q, got result %v", tc.wantErr, resp.Result.Value())
				}
				if !strings.Contains(resp.Error.Text, tc.wantErr) {
					t.Fatalf("error %q does not contain %q", resp.Error.Text, tc.wantErr)
				}
				if resp.Error.FunctionArgument == nil || *resp.Error.FunctionArgument != tc.wantArg {
					t.Fatalf("error argument = %v, want %d", resp.Error.FunctionArgument, tc.wantArg)
				}
				return
			}
			if resp.Error != nil {
				t.Fatalf("unexpected error: %s", resp.Error.Text)
			}
			if got := resp.Result.Value(); !got.Equal(types.StringValue(tc.want)) {
				t.Fatalf("got %v, want %q", got, tc.want)
			}
		})
	}
}

func TestFunctionConsoleAddressRun(t *testing.T) {
	s, i := types.StringValue, types.Int64Value
	runFunctionCases(t, newConsoleAddressFunction(), []funcCase{
		{name: "ipv4", args: []attr.Value{s("127.0.0.1"), i(5000)}, want: "127.0.0.1:5000"},
		{name: "hostname", args: []attr.Value{s("gns3.lab"), i(5001)}, want: "gns3.lab:5001"},
		{name: "ipv6", args: []attr.Value{s("::1"), i(5000)}, want: "[::1]:5000"},
		{name: "port 1", args: []attr.Value{s("h"), i(1)}, want: "h:1"},
		{name: "port 65535", args: []attr.Value{s("h"), i(65535)}, want: "h:65535"},
		{name: "empty host", args: []attr.Value{s(""), i(5000)}, wantErr: "host must not be empty", wantArg: 0},
		{name: "port 0", args: []attr.Value{s("h"), i(0)}, wantErr: "between 1 and 65535, got 0", wantArg: 1},
		{name: "port negative", args: []attr.Value{s("h"), i(-1)}, wantErr: "between 1 and 65535, got -1", wantArg: 1},
		{name: "port 65536", args: []attr.Value{s("h"), i(65536)}, wantErr: "between 1 and 65535, got 65536", wantArg: 1},
	})
}

func TestFunctionNodeNameRun(t *testing.T) {
	s, i := types.StringValue, types.Int64Value
	runFunctionCases(t, newNodeNameFunction(), []funcCase{
		{name: "zero", args: []attr.Value{s("r"), i(0)}, want: "r-00"},
		{name: "one", args: []attr.Value{s("r"), i(1)}, want: "r-01"},
		{name: "two digits", args: []attr.Value{s("r"), i(12)}, want: "r-12"},
		{name: "three digits", args: []attr.Value{s("r"), i(100)}, want: "r-100"},
		{name: "prefix with dash", args: []attr.Value{s("core-sw"), i(3)}, want: "core-sw-03"},
		{name: "empty prefix", args: []attr.Value{s(""), i(1)}, wantErr: "prefix must not be empty", wantArg: 0},
		{name: "negative index", args: []attr.Value{s("r"), i(-1)}, wantErr: "index must not be negative, got -1", wantArg: 1},
	})
}

// TestFunctionsConfig calls the functions from a Terraform configuration. The
// provider block points at the fake server because Terraform configures the
// provider before evaluating outputs; the functions themselves never call it.
func TestFunctionsConfig(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + `
output "console" {
  value = provider::gns3::console_address("127.0.0.1", 5000)
}
output "console_v6" {
  value = provider::gns3::console_address("::1", 5001)
}
output "names" {
  value = [for i in [1, 12, 100] : provider::gns3::node_name("r", i)]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("console", knownvalue.StringExact("127.0.0.1:5000")),
					statecheck.ExpectKnownOutputValue("console_v6", knownvalue.StringExact("[::1]:5001")),
					statecheck.ExpectKnownOutputValue("names", knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("r-01"), knownvalue.StringExact("r-12"), knownvalue.StringExact("r-100"),
					})),
				},
			},
			{
				Config:      env.providerBlock + "output \"bad\" {\n  value = provider::gns3::console_address(\"\", 5000)\n}\n",
				ExpectError: regexp.MustCompile(`host must not be empty`),
			},
			{
				Config:      env.providerBlock + "output \"bad\" {\n  value = provider::gns3::node_name(\"r\", -1)\n}\n",
				ExpectError: regexp.MustCompile(`index must not be negative`),
			},
		},
	})
}
