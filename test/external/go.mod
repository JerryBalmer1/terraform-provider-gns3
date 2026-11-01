// A module outside terraform-provider-gns3 that imports pkg/gns3, to prove the
// client is importable cross-module. Run by pkg/gns3/importable_test.go.
module example.com/gns3-external

go 1.25.8

require github.com/JerryBalmer1/terraform-provider-gns3 v0.0.0

replace github.com/JerryBalmer1/terraform-provider-gns3 => ../..
