# terraform-provider-gns3

A Terraform provider for [GNS3](https://www.gns3.com/): projects, nodes and links on a GNS3 server, managed through its REST API. Registry address `JerryBalmer1/gns3`, version 0.1.0.

## GNS3 version

Built against the **GNS3 server 2.2.52** controller API (REST v2, `/v2/...`), as documented at <https://gns3-server.readthedocs.io/en/stable/endpoints.html>. (`docs.gns3.com/apiv2` returned 404 when this was written; the readthedocs pages are the same reference.) Each client method in `internal/client/` cites its endpoint page.

It has been run against a live **GNS3 2.2.61** server on Windows (2026-10-07): every acceptance test passes, `manual-check-list.md` section 1 has been seen, and the client fixtures are recorded from that server. Where 2.2.61 differs from the 2.2.52 docs, [docs/api-drift.md](docs/api-drift.md) lists the difference and which one the client follows.

## Requirements

- Terraform 1.0+ (tested here with 1.9.8)
- Go 1.25.8+ to build (the floor set by terraform-plugin-framework 1.19 and terraform-plugin-testing 1.16)
- A GNS3 server 2.2.x reachable over HTTP(S)

## Get a server

For Windows with nothing installed. GNS3 is not on winget; the Windows build is the all-in-one installer attached to each release at <https://github.com/GNS3/gns3-gui/releases> (`GNS3-<version>-all-in-one.exe`, with a `.sha256` file beside it). It installs the GUI, the server and extras (Npcap, Wireshark, Dynamips, QEMU, Solar-PuTTY, ...); the provider needs only the server and VPCS.

With [Invoke-Build](https://github.com/nightroman/Invoke-Build) (`Install-Module InvokeBuild -Scope CurrentUser`), from the repo root:

1. **Install:** `Invoke-Build Gns3Install` (latest release; `-Version 2.2.61` for a specific one). It downloads the installer and its `.sha256` to `$env:TEMP\gns3`, refuses on a hash mismatch, runs the installer silently (accept the UAC prompt) and records the result in `docs/gns3-install.md`.
2. **Start:** `Invoke-Build Gns3Start`. It runs `gns3server.exe` in the background on `127.0.0.1:3080` with its own config under `$env:TEMP\gns3\config`, so your GUI settings and projects are untouched. `Invoke-Build Gns3Stop` stops it again (only that server, never one started from the GUI).
3. **Verify:** `Invoke-Build Gns3Status` prints the installed version, the server process and port, and what `GET /v2/version` answers.

`Invoke-Build TestAcc -AutoStart` does steps 2 and 3 itself, runs the acceptance tests, and stops the server afterwards if it started it.

Without Invoke-Build, the same three steps by hand in PowerShell:

1. **Install:**

   ```powershell
   $v = '2.2.61'
   $base = "https://github.com/GNS3/gns3-gui/releases/download/v$v/GNS3-$v-all-in-one.exe"
   $dir = New-Item -ItemType Directory -Force (Join-Path $env:TEMP 'gns3')
   Invoke-WebRequest "$base.sha256" -OutFile "$dir\installer.sha256"
   Invoke-WebRequest $base -OutFile "$dir\GNS3-$v-all-in-one.exe"
   $want = ((Get-Content "$dir\installer.sha256" -Raw).Trim() -split '\s+')[0]
   if ((Get-FileHash "$dir\GNS3-$v-all-in-one.exe").Hash -ne $want) { throw 'SHA-256 mismatch' }
   Start-Process "$dir\GNS3-$v-all-in-one.exe" -ArgumentList '/S' -Verb RunAs -Wait
   ```

2. **Start:** start GNS3 from the Start menu and finish (or skip) its setup wizard; the GUI runs its own local server on `127.0.0.1:3080` while it is open. To run the server without the GUI, write a config like the one `Gns3Start` writes (`[Server]` host, port and paths, `[VPCS] vpcs_path = C:\Program Files\GNS3\vpcs\vpcs.exe`, `[Dynamips] dynamips_path = C:\Program Files\GNS3\dynamips\dynamips.exe`; Windows runs the built-in switch and hub on Dynamips) and run `& 'C:\Program Files\GNS3\gns3server.exe' --config <that file> --local`.
3. **Verify:** `Invoke-RestMethod http://127.0.0.1:3080/v2/version` prints `version` and `local`.

## Provider configuration

```terraform
provider "gns3" {
  host = "127.0.0.1" # GNS3_HOST
  port = 3080        # GNS3_PORT
}
```

| Argument | Environment | Default | Notes |
|---|---|---|---|
| `host` | `GNS3_HOST` | `127.0.0.1` | |
| `port` | `GNS3_PORT` | `3080` | |
| `scheme` | | `http` | `http` or `https` |
| `username` | `GNS3_USERNAME` | | HTTP Basic auth (`auth = True` in `gns3_server.conf`) |
| `password` | `GNS3_PASSWORD` | | Sensitive |
| `token` | `GNS3_TOKEN` | | Sensitive. For the GNS3 3.x v3 API; ignored by this release with a warning |
| `insecure_skip_verify` | | `false` | Skip TLS verification for `https` |

A value set in the provider block wins over its environment variable, and the environment variable wins over the default. When the provider is configured it calls `GET /v2/version` and fails with the address and the error if the server can't be reached.

## Resources and data sources

| Type | Name | Docs |
|---|---|---|
| Resource | `gns3_project` | [docs/resources/project.md](docs/resources/project.md) |
| Resource | `gns3_node` | [docs/resources/node.md](docs/resources/node.md) |
| Resource | `gns3_link` | [docs/resources/link.md](docs/resources/link.md) |
| Data source | `gns3_template` | [docs/data-sources/template.md](docs/data-sources/template.md) |
| Data source | `gns3_compute` | [docs/data-sources/compute.md](docs/data-sources/compute.md) |
| Data source | `gns3_project` | [docs/data-sources/project.md](docs/data-sources/project.md) |
| Data source | `gns3_version` | [docs/data-sources/version.md](docs/data-sources/version.md) |

`docs/` is generated by tfplugindocs from the schema and `examples/`; never edit it by hand. Run `make docs`.

A two-PC lab with a link between them, using only the built-in VPCS template:

```terraform
resource "gns3_project" "lab" {
  name = "lab"
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc" {
  for_each        = { PC1 = -150, PC2 = 150 }
  project_id      = gns3_project.lab.project_id
  template_id     = data.gns3_template.vpcs.template_id
  name            = each.key
  x               = each.value
  start_on_create = true
}

resource "gns3_link" "pc1_pc2" {
  project_id = gns3_project.lab.project_id
  node {
    node_id        = gns3_node.pc["PC1"].node_id
    adapter_number = gns3_node.pc["PC1"].ports[0].adapter_number
    port_number    = gns3_node.pc["PC1"].ports[0].port_number
  }
  node {
    node_id        = gns3_node.pc["PC2"].node_id
    adapter_number = gns3_node.pc["PC2"].ports[0].adapter_number
    port_number    = gns3_node.pc["PC2"].ports[0].port_number
  }
}
```

### Behaviour worth knowing

- Every API error is reported with the HTTP method, the endpoint, the HTTP status and the server's message, for example `GNS3 API POST /v2/projects/<id>/links returned HTTP 409: Cannot connect to itself`.
- `gns3_project` is created opened. Destroying it closes the project first and then deletes it from disk.
- A closed project hides its nodes and links from the API. Refreshing a node or link in a closed project fails with "GNS3 project … is closed" and leaves state alone. Open the project, or set `auto_open = true`.
- `gns3_node.properties` takes a JSON object. Only the keys you set are compared with the server, so emulator defaults you never set don't show up as drift.
- A VPCS node's `startup_script` is written to and read from its `startup.vpc` file, not sent as a property: GNS3 2.2 rejects it in node updates (see [docs/api-drift.md](docs/api-drift.md)).
- Destroying a project retries for up to 15 seconds while a Windows server reports "Cannot delete project directory … being used by another process", which happens when a just-deleted node's emulator has not exited yet.
- `start_on_create` only acts when the node is created. Use the GNS3 GUI or API to start and stop nodes after that (see "Not here yet").
- `ports` is read back from the server after create, so `gns3_link` can reference `gns3_node.x.ports[n]`.

## Local development loop

Build and install the provider into the Go bin directory:

```powershell
Set-Location C:\__Code\terraform-provider-gns3
go install .
```

Point Terraform at that directory with `dev_overrides`. On Windows the CLI config file is `%APPDATA%\terraform.rc`; on Linux and macOS it is `~/.terraformrc`. This PowerShell writes it, keeping a copy of any existing file as `terraform.rc.bak`:

```powershell
$bin = go env GOBIN
if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$rc = Join-Path $env:APPDATA 'terraform.rc'
if (Test-Path $rc) { Copy-Item $rc "$rc.bak" -Force }
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $rc -Encoding ascii
```

The equivalent `~/.terraformrc` on Linux or macOS:

```hcl
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "/home/<you>/go/bin"
  }
  direct {}
}
```

Then, in any directory whose configuration has `source = "JerryBalmer1/gns3"` (see `examples/provider/provider.tf`), run `terraform plan` or `terraform apply` directly. Terraform prints "Provider development overrides are in effect" and uses the local build. Rerun `go install .` after every change.

Under a dev override:

- **Don't run `terraform init`.** It still asks the registry for `jerrybalmer1/gns3`, which isn't published yet, and fails with "Failed to query available provider packages".
- **`terraform providers mirror` ignores `dev_overrides`.** It always downloads from the registry, so it fails the same way. `terraform providers` and `terraform providers schema -json` do use the override.

To get the old behaviour back, delete the `dev_overrides` block (or restore `terraform.rc.bak`).

## Make targets

`GNUmakefile` has `build`, `install`, `test`, `testacc`, `docs`, `lint` and `fmt`, and stays the cross-platform path. On Windows, `.build.ps1` has the same commands as Invoke-Build tasks (`Build`, `Install`, `Test`, `Lint`, `Docs`; `Invoke-Build` alone runs `Build`, `Lint`, `Test`), plus `Gns3Status`, `Gns3Install`, `Gns3Start`, `Gns3Stop`, `TestAcc [-AutoStart]` and `Checklist -Item <n.m>`, which runs one `manual-check-list.md` item in a fresh pwsh and prints its output beside its Expect line. Without either, run the commands directly:

| Target | Command |
|---|---|
| `make build` | `go build -o . ./...` |
| `make install` | `go install .` |
| `make test` | `go test ./... -count=1 -timeout 10m` |
| `make testacc` | `$env:TF_ACC = '1'; go test ./internal/provider/ -run TestAcc -count=1 -v -timeout 60m` |
| `make docs` | `go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name gns3`, then `... validate --provider-name gns3` |
| `make lint` | `go vet ./...`; `go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...`; `gofmt -l .` (must print nothing) |

## Tests

- **Client unit tests** (`internal/client/*_test.go`): each client method is replayed against `httptest` from a fixture in `internal/client/testdata/<area>/<name>.json`. The harness checks the method, the path and the JSON request body. Each fixture names its provenance in `_header` and its docs page in `_source`. Every fixture is `recorded from GNS3 2.2.61 on 2026-10-07` against a local Windows server; `_notes` says how a recording was set up when that is not obvious. Node file fixtures carry `text` instead of `body` for their raw, non-JSON content.
- **Provider unit tests** (`internal/provider/*_test.go`, `Test*` without `Acc`): the same create, update, import, replace and destroy steps as the acceptance tests, run with `resource.UnitTest` against the in-memory fake in `fakeserver_test.go`. They drive the real Terraform CLI on `PATH`. The `TestExamples` test applies every `examples/resources/*/resource.tf` and requires an empty plan afterwards.
- **Acceptance tests** (`TestAcc*`, run only with `TF_ACC=1`): the same steps against the live server in `GNS3_HOST`/`GNS3_PORT`/... . They create projects named `tf-acc-*`, plus `tf-example-project`, `tf-example-node` and `tf-example-link` from the examples, and destroy them at the end. `TestAccExamples` is the idempotency test: a fresh apply of each example must leave an empty plan.

## Not here yet

| Feature | Why |
|---|---|
| `gns3_template` resource | Template bodies differ by emulator type (qemu, iou, dynamips, docker, ...), so each needs its own schema. 0.1.0 reads templates with the data source only. |
| `gns3_drawing` | Not needed for topology. Drawings are SVG blobs with their own endpoints. |
| Snapshots | Snapshot create and restore are actions rather than desired state, and restore rewrites every node. They need a design first. |
| Project export/import | These are file transfers (`/export`, `/import` stream a .gns3project archive), not resources. |
| GNS3 3.x / v3 API | The client interface (`client.Client`) is version-neutral and `token` is already in the provider schema. A v3 implementation (JWT auth, `/v3/...`) would be a second type behind the same interface. Nothing in 0.1.0 talks to v3. |
| Node start/stop as a separate resource | Run state is an action on a node, not one of its attributes. `start_on_create` covers the first start; a separate resource (or action) for start, stop and suspend comes later. |

## Release

`.goreleaser.yml` builds the Registry layout (zip per OS/arch, `SHA256SUMS`, manifest) and signs the checksum file with GPG (`GPG_FINGERPRINT`). Nothing has been released and no tag exists; the author cuts releases.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
