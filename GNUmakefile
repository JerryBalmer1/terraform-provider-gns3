default: build

GO ?= go
# Pinned via go run so an old staticcheck on PATH cannot reject the go 1.25 module.
STATICCHECK ?= $(GO) run honnef.co/go/tools/cmd/staticcheck@v0.7.0

.PHONY: build install test testacc docs lint fmt

# Build the provider binary into the repo root.
build:
	$(GO) build -o . ./...

# Install into $(go env GOBIN) or $(go env GOPATH)/bin, the dev_overrides path.
install:
	$(GO) install .

# Client fixture tests and provider tests against the in-memory fake server.
# Provider tests drive the terraform CLI on PATH.
test:
	$(GO) test ./... -count=1 -timeout 10m

# Acceptance tests against a live GNS3 2.2 server (GNS3_HOST, GNS3_PORT, ...).
# Creates and destroys projects named tf-acc-* and tf-example-*.
testacc:
	TF_ACC=1 $(GO) test ./internal/provider/ -run TestAcc -count=1 -v -timeout 60m

# Regenerate docs/ from the schema, examples/ and templates/ (never edit docs/ by hand).
docs:
	$(GO) run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name gns3
	$(GO) run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate --provider-name gns3

# go vet, staticcheck, and a gofmt check that fails on any unformatted file.
# staticcheck v0.7.0 cannot read export data from newer Go releases (1.27), so it
# runs under the go.mod toolchain; GOTOOLCHAIN is set for that command only.
lint:
	$(GO) vet ./...
	GOTOOLCHAIN=go1.25.8 $(STATICCHECK) ./...
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

fmt:
	gofmt -w .
