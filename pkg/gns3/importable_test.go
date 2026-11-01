package gns3

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestImportableFromOtherModule runs the tests of test/external, a separate
// module that imports this package through a replace to the repo root. A
// nested module is outside `go test ./...`, so this test runs it. It fails if
// the package stops being importable cross-module (e.g. moved under internal/).
func TestImportableFromOtherModule(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "test", "external"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	// GOFLAGS=-mod=... from the parent run would apply to the other module too.
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test in test/external failed: %v\n%s", err, out)
	}
	t.Logf("%s", out)
}
