# terraform-provider-gns3 manual check list

Provider version: 0.2.0
Built against: GNS3 server 2.2.52 docs (REST v2); live-checked against GNS3 server 2.2.61 on 2026-10-07 (see docs/api-drift.md)
Last updated: 2026-10-15

Append-only: never renumber items. New items go at the end of their section; a withdrawn item keeps its number with "(removed in 0.x.0)". Every block is self-contained. It installs the provider with `go install .` and writes its own dev-override CLI config under `$env:TEMP` through `TF_CLI_CONFIG_FILE`, so `%APPDATA%\terraform.rc` is never read or changed. An Expect marked "(not yet seen: needs a GNS3 server)" is what the code and docs predict; replace the marker once you have seen it.

## 0 Setup

### 0.1 Version and exported types

Builds the provider, prints its version, and lists the resource and data source types it exports.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go install .
$bin = go env GOBIN; if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$env:TF_CLI_CONFIG_FILE = Join-Path $env:TEMP 'gns3-dev.tfrc'
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $env:TF_CLI_CONFIG_FILE -Encoding ascii
& (Join-Path $bin 'terraform-provider-gns3.exe') -version
$d = Join-Path $env:TEMP 'gns3-check-0.1'; New-Item -ItemType Directory -Force $d | Out-Null
Copy-Item .\examples\provider\provider.tf $d
Push-Location $d
$s = (terraform providers schema -json | ConvertFrom-Json).provider_schemas.'registry.terraform.io/jerrybalmer1/gns3'
'resources:    ' + (($s.resource_schemas.PSObject.Properties.Name | Sort-Object) -join ', ')
'data sources: ' + (($s.data_source_schemas.PSObject.Properties.Name | Sort-Object) -join ', ')
Pop-Location; Remove-Item -Recurse -Force $d
```

Expect: `0.2.0`, then `resources:    gns3_image, gns3_link, gns3_node, gns3_project` and `data sources: gns3_compute, gns3_image, gns3_project, gns3_template, gns3_version` (seen 2026-10-15; `gns3_image` added in both lists; version bumped to 0.2.0 on 2026-10-15).

Tests: none (the schema listing is not asserted in Go; `TestExamples` loads every resource type)

## 1 Provider against a local GNS3 2.2 server

Every item in this section needs a GNS3 2.2.x server on `127.0.0.1:3080`, or `GNS3_HOST`/`GNS3_PORT` set. Items 1.2 to 1.5 create projects named `tf-example-*` and destroy them before they finish.

### 1.1 Build and resolve through the dev override

Builds the provider and confirms Terraform resolves `JerryBalmer1/gns3` to the local build without `terraform init`.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go build ./...
go install .
$bin = go env GOBIN; if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$env:TF_CLI_CONFIG_FILE = Join-Path $env:TEMP 'gns3-dev.tfrc'
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $env:TF_CLI_CONFIG_FILE -Encoding ascii
$d = Join-Path $env:TEMP 'gns3-check-1.1'; New-Item -ItemType Directory -Force $d | Out-Null
Copy-Item .\examples\provider\provider.tf $d
Copy-Item .\examples\data-sources\gns3_version\data-source.tf $d
Push-Location $d
terraform providers
terraform plan
Pop-Location; Remove-Item -Recurse -Force $d
```

Expect: `go build` prints nothing. `terraform providers` lists `provider[registry.terraform.io/jerrybalmer1/gns3] ~> 0.1`. `terraform plan` warns "Provider development overrides are in effect" naming your Go bin directory, then shows `Changes to Outputs: + gns3_version = "2.2.<n>"` (seen 2026-10-07 against GNS3 2.2.61: `+ gns3_version = "2.2.61"`). With no server running, the plan instead fails with "Cannot reach the GNS3 server … GET /v2/version" (seen 2026-10-07 on Terraform 1.9.8). `terraform providers mirror` is left out on purpose: it ignores `dev_overrides`, always asks the registry, and fails with "Provider not available" until the provider is published (seen 2026-10-07 on Terraform 1.9.8).

Tests: "TestProviderUnreachableServer"

### 1.2 gns3_project apply and re-plan

Applies `examples/resources/gns3_project`, re-plans, and destroys.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go install .
$bin = go env GOBIN; if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$env:TF_CLI_CONFIG_FILE = Join-Path $env:TEMP 'gns3-dev.tfrc'
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $env:TF_CLI_CONFIG_FILE -Encoding ascii
$d = Join-Path $env:TEMP 'gns3-check-1.2'; New-Item -ItemType Directory -Force $d | Out-Null
Copy-Item .\examples\provider\provider.tf, .\examples\resources\gns3_project\resource.tf $d
Push-Location $d
terraform apply -auto-approve
terraform plan
terraform destroy -auto-approve
Pop-Location; Remove-Item -Recurse -Force $d
```

Expect: apply ends `Apply complete! Resources: 1 added, 0 changed, 0 destroyed.` and project `tf-example-project` appears in the GNS3 GUI. `terraform plan` prints `No changes. Your infrastructure matches the configuration.` Destroy ends `Destroy complete! Resources: 1 destroyed.` and the project directory is gone from disk (seen 2026-10-07 against GNS3 2.2.61 via `Invoke-Build Checklist -Item 1.2`).

Tests: "TestExamples/gns3_project", "TestProjectResource", "TestAccExamples/gns3_project", "TestAccProjectResource"

### 1.3 gns3_node from a built-in template

Applies `examples/resources/gns3_node` (one VPCS node from the built-in template, one template-less VPCS node; no image download), re-plans, and destroys.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go install .
$bin = go env GOBIN; if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$env:TF_CLI_CONFIG_FILE = Join-Path $env:TEMP 'gns3-dev.tfrc'
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $env:TF_CLI_CONFIG_FILE -Encoding ascii
$d = Join-Path $env:TEMP 'gns3-check-1.3'; New-Item -ItemType Directory -Force $d | Out-Null
Copy-Item .\examples\provider\provider.tf, .\examples\resources\gns3_node\resource.tf $d
Push-Location $d
terraform apply -auto-approve
terraform state show gns3_node.pc1
terraform plan
terraform destroy -auto-approve
Pop-Location; Remove-Item -Recurse -Force $d
```

Expect: apply ends `3 added`, and project `tf-example-node` shows PC1 and PC2 in the GUI. `state show` lists `node_type = "vpcs"`, `status = "stopped"`, a numeric `console` (5002), `symbol = ":/symbols/vpcs_guest.svg"` and `ports` with one entry: `adapter_number = 0`, `port_number = 0`, `name = "Ethernet0"`, `link_type = "ethernet"`. `terraform plan` prints `No changes.`: `pc2.properties` keeps `startup_script = "set pcname PC2"`, read back from the node's `startup.vpc` file. Destroy ends `3 destroyed` (seen 2026-10-07 against GNS3 2.2.61).

Tests: "TestExamples/gns3_node", "TestNodeResource", "TestAccExamples/gns3_node", "TestAccNodeResource"

### 1.4 gns3_link between two VPCS nodes, then ping

Applies `examples/resources/gns3_link` (two started VPCS nodes and a link), sets addresses and pings across through the VPCS consoles, then destroys.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go install .
$bin = go env GOBIN; if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$env:TF_CLI_CONFIG_FILE = Join-Path $env:TEMP 'gns3-dev.tfrc'
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $env:TF_CLI_CONFIG_FILE -Encoding ascii
$d = Join-Path $env:TEMP 'gns3-check-1.4'; New-Item -ItemType Directory -Force $d | Out-Null
Copy-Item .\examples\provider\provider.tf, .\examples\resources\gns3_link\resource.tf $d
Push-Location $d
terraform apply -auto-approve
terraform plan
$consoles = @{}
(terraform show -json | ConvertFrom-Json).values.root_module.resources | Where-Object type -eq 'gns3_node' |
    ForEach-Object { $consoles[$_.values.name] = $_.values.console }
# Types commands into a VPCS telnet console (the same one the GUI opens) and prints its answers.
function Invoke-VpcsConsole([int] $Port, [string[]] $Commands, [int] $WaitMs) {
    $tcp = [Net.Sockets.TcpClient]::new('127.0.0.1', $Port)
    $stream = $tcp.GetStream(); $buffer = [byte[]]::new(65536); $text = [Text.StringBuilder]::new()
    $drain = {
        Start-Sleep -Milliseconds $WaitMs
        while ($stream.DataAvailable) {
            $n = $stream.Read($buffer, 0, $buffer.Length)
            for ($i = 0; $i -lt $n; $i++) {
                if ($buffer[$i] -eq 255) { $i += 2; continue }   # telnet IAC negotiation
                if ($buffer[$i] -ge 32 -or $buffer[$i] -eq 10) { [void]$text.Append([char]$buffer[$i]) }
            }
        }
    }
    & $drain
    foreach ($command in $Commands) {
        $bytes = [Text.Encoding]::ASCII.GetBytes("$command`r`n"); $stream.Write($bytes, 0, $bytes.Length)
        & $drain
    }
    $tcp.Close()
    $text.ToString() -split "`n" | Where-Object { $_.Trim() }
}
"--- PC1 console (port $($consoles['PC1']))"
Invoke-VpcsConsole -Port $consoles['PC1'] -Commands 'ip 10.0.0.1/24' -WaitMs 3000
"--- PC2 console (port $($consoles['PC2']))"
Invoke-VpcsConsole -Port $consoles['PC2'] -Commands 'ip 10.0.0.2/24', 'ping 10.0.0.1' -WaitMs 7000
terraform destroy -auto-approve
Pop-Location; Remove-Item -Recurse -Force $d
```

Expect: apply ends `4 added`; PC1 and PC2 are started with a link between their `e0` ports. `terraform plan` prints `No changes.` The block then types into the two VPCS telnet consoles itself (the same consoles the GUI opens): PC1 answers `PC1> ip 10.0.0.1/24` / `Checking for duplicate address...`; PC2 answers `PC1 : 10.0.0.2 255.255.255.0` (VPCS names every instance PC1 in its own prompt line) and `ping 10.0.0.1` prints five lines `84 bytes from 10.0.0.1 icmp_seq=1 ttl=64 time=0.625 ms` … `icmp_seq=5`. Destroy ends `4 destroyed` (seen 2026-10-07 against GNS3 2.2.61; the project DELETE got one 409 "Cannot delete project directory … [WinError 32]" and the retry succeeded, see docs/api-drift.md).

Tests: "TestExamples/gns3_link", "TestLinkResource", "TestAccExamples/gns3_link", "TestAccLinkResource" (none of them pings)

### 1.5 terraform import of each resource

Applies the link example, drops all three resource types from state, imports them back by their documented IDs, re-plans, and destroys.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go install .
$bin = go env GOBIN; if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$env:TF_CLI_CONFIG_FILE = Join-Path $env:TEMP 'gns3-dev.tfrc'
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $env:TF_CLI_CONFIG_FILE -Encoding ascii
$d = Join-Path $env:TEMP 'gns3-check-1.5'; New-Item -ItemType Directory -Force $d | Out-Null
Copy-Item .\examples\provider\provider.tf, .\examples\resources\gns3_link\resource.tf $d
Push-Location $d
terraform apply -auto-approve
$state = terraform show -json | ConvertFrom-Json
$r  = @{}; $state.values.root_module.resources | ForEach-Object { $r[$_.address] = $_.values }
$pid_ = $r['gns3_project.lab'].project_id
terraform state rm gns3_link.pc1_pc2 gns3_node.pc1 gns3_node.pc2 gns3_project.lab
terraform import gns3_project.lab $pid_
terraform import gns3_node.pc1 "$pid_/$($r['gns3_node.pc1'].node_id)"
terraform import gns3_node.pc2 "$pid_/$($r['gns3_node.pc2'].node_id)"
terraform import gns3_link.pc1_pc2 "$pid_/$($r['gns3_link.pc1_pc2'].link_id)"
terraform plan
terraform destroy -auto-approve
Pop-Location; Remove-Item -Recurse -Force $d
```

Expect: four `Import successful!` messages. Then `terraform plan` shows only `start_on_create` going from `false` to `true` on `gns3_node.pc1` and `gns3_node.pc2` (`Plan: 0 to add, 2 to change, 0 to destroy.`), as in-place updates that make no API call. Import can't know a create-time flag; every other attribute matches. Destroy ends `4 destroyed` (seen 2026-10-07 against GNS3 2.2.61).

Tests: "TestProjectResource", "TestNodeResource", "TestLinkResource" (their ImportState steps), and the matching TestAcc* tests

### 1.6 tfplugindocs validate

Regenerates `docs/` and validates it.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name gns3
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate --provider-name gns3
$LASTEXITCODE
git diff --name-only -- docs
```

Expect: generate ends `rendering "resources\\project.md.tmpl"`. Validate prints its checks (`running mixed directories check`, `running file mismatch check`, ...) with no error lines, and `$LASTEXITCODE` is `0`. `git diff --name-only -- docs` prints nothing, which means the regenerated docs match what is staged or committed (seen 2026-10-07).

Tests: none

### 1.7 Dogfood: the provider schema as a TerraformGraph input

Reads this provider's schema through the dev override and builds a TerraformGraph schema graph from it.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go install .
$bin = go env GOBIN; if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$env:TF_CLI_CONFIG_FILE = Join-Path $env:TEMP 'gns3-dev.tfrc'
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $env:TF_CLI_CONFIG_FILE -Encoding ascii
$d = Join-Path $env:TEMP 'gns3-check-1.7'; New-Item -ItemType Directory -Force $d | Out-Null
Copy-Item .\examples\provider\provider.tf $d
Remove-Module TerraformAST, TerraformGraph -Force -ErrorAction SilentlyContinue
Import-Module C:\__Code\TerraformGraph\src\TerraformGraph\TerraformGraph.psd1
$g = Get-TerraformProviderSchema -Path $d | ConvertTo-TerraformSchemaGraph
'Nodes: ' + $g.Nodes.Count
'Edges: ' + $g.Edges.Count
$g.Nodes | Group-Object Kind | Select-Object Count, Name
Remove-Item -Recurse -Force $d
```

Expect: `Nodes: 93`, `Edges: 92`, grouped as 82 Attribute, 1 Block, 5 DataSource, 1 Provider, 4 Resource (seen 2026-10-15 with the `gns3_image` resource and data source; before them it was 77 nodes and 76 edges, seen 2026-10-07 against TerraformGraph 0.14.0). No GNS3 server is needed. `Get-TerraformProviderSchema -Path` never runs `terraform init`, which would fail under a dev override.

Tests: none

### 1.8 GNS3 server status

Reports whether GNS3 is installed (uninstall key or `C:\Program Files\GNS3`), its version, whether `gns3server` runs and on which port, and what `GET /v2/version` answers. Installs and starts nothing.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
Invoke-Build Gns3Status
```

Expect: `Installed : True`, `InstalledVersion : 2.2.61`, `InstallFolder : C:\Program Files\GNS3`, `FoundBy : registry: HKEY_LOCAL_MACHINE\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\GNS3`. With a server up (`Invoke-Build Gns3Start`): `ServerRunning : True`, `ListeningOn : 127.0.0.1:3080`, `Answers : True`, `ApiVersion : 2.2.61`, `ApiLocal : True`, and `StartedByBuild` shows the PID when Gns3Start started it. With no server: `ServerRunning : False`, `Answers : False` (both seen 2026-10-07).

Tests: none

### 1.9 Acceptance tests through the build

Runs every `TestAcc*` test against a live server, starting one with `Gns3Start` when nothing answers and stopping it afterwards only if this run started it. The log is `acc-run-<timestamp>.log` in the repo root (gitignored).

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
Invoke-Build TestAcc -AutoStart
```

Expect: eleven `--- PASS: TestAcc…` lines (`TestAccComputeDataSource`, `TestAccImageDataSource`, `TestAccImageResource`, `TestAccImageResourceSourceURL`, `TestAccLinkResource`, `TestAccNodeResource`, `TestAccProjectDataSource`, `TestAccProjectResource`, `TestAccExamples`, `TestAccTemplateDataSource`, `TestAccVersionDataSource`). `TestAccExamples` passes `gns3_link`, `gns3_node` and `gns3_project`, and skips `gns3_image` with "example not applied: downloads a ~612 MiB VyOS ISO …". Then `ok  github.com/JerryBalmer1/terraform-provider-gns3/internal/provider` and `Acceptance tests passed. Log: …\acc-run-<timestamp>.log` (seen 2026-10-15 against GNS3 2.2.61; the test binary took 33 s). The three image tests leave `tf-acc-image*.img` files in the server's QEMU images folder, because GNS3 2.2 cannot delete images. With `-AutoStart` that folder is `$env:TEMP\gns3\data\images\QEMU`.

Tests: every TestAcc* test
### 1.10 Images on a compute

Lists the qemu, dynamips and iou images on the local compute, and the Docker images its daemon has, through the controller API (`Invoke-Build Gns3Images`). Downloads and uploads nothing.

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
Invoke-Build Gns3Images
```

Expect: `iou: HTTP 404, GET /iou/images not found (this compute has no iou support, or the compute does not exist)` in yellow, then `Images on compute 'local' at http://127.0.0.1:3080:` and a table with the columns `Emulator Filename SizeMB MD5 Path`. On a fresh 2.2.61 server the table has 16 qemu rows: `OVMF_CODE.fd`, `OVMF_CODE_4M.fd`, `OVMF_VARS.fd`, `OVMF_VARS_4M.fd` and `empty8G.qcow2` … `empty1T.qcow2`. `empty8G.qcow2` has MD5 `bfd65172dea027c3fc2eb5dfa3a7c2b9`. Files uploaded earlier, such as `tf-acc-image*.img`, add rows. There are no dynamips rows. The last line is `docker: HTTP 404, GET /docker/images not found …`, because a Windows compute has no IOU or Docker (seen 2026-10-15 against GNS3 2.2.61 on Windows, started by `Gns3Start`). On a GNS3 VM compute (`-ComputeId vm`), iou images would be table rows and docker would print a list of `repo:tag` names (not yet seen: needs a GNS3 VM).

Tests: "TestListImages", "TestListImagesEmpty", "TestListDockerImages" (client fixtures; the task itself is not tested in Go)

### 1.11 gns3_image upload, data source, destroy

Uploads a small generated file with `gns3_image`, reads it back with the `gns3_image` data source, re-plans, destroys, and shows that the file is still on the compute (GNS3 2.2 cannot delete images).

```powershell
Set-Location 'C:\__Code\terraform-provider-gns3'
go install .
$bin = go env GOBIN; if (-not $bin) { $bin = Join-Path (go env GOPATH) 'bin' }
$env:TF_CLI_CONFIG_FILE = Join-Path $env:TEMP 'gns3-dev.tfrc'
@"
provider_installation {
  dev_overrides {
    "JerryBalmer1/gns3" = "$($bin -replace '\\', '/')"
  }
  direct {}
}
"@ | Set-Content -Path $env:TF_CLI_CONFIG_FILE -Encoding ascii
$d = Join-Path $env:TEMP 'gns3-check-1.11'; New-Item -ItemType Directory -Force $d | Out-Null
Copy-Item .\examples\provider\provider.tf $d
$name = "tf-check-$(Get-Date -Format 'yyyyMMddHHmmss').img"
$src = Join-Path $d 'image.img'
Set-Content -LiteralPath $src -Value "manual check image $name" -NoNewline -Encoding ascii
"md5 of the source: $((Get-FileHash $src -Algorithm MD5).Hash.ToLower())"
@"
resource "gns3_image" "check" {
  emulator    = "qemu"
  filename    = "$name"
  source_path = "$($src -replace '\\', '/')"
  md5sum      = filemd5("$($src -replace '\\', '/')")
}

data "gns3_image" "check" {
  emulator = "qemu"
  filename = gns3_image.check.filename
}

output "listed" {
  value = "`${data.gns3_image.check.path} `${data.gns3_image.check.filesize} `${data.gns3_image.check.md5sum}"
}
"@ | Set-Content -Path (Join-Path $d 'main.tf') -Encoding ascii
Push-Location $d
terraform apply -auto-approve
terraform plan
terraform destroy -auto-approve
Pop-Location; Remove-Item -Recurse -Force $d
$gns3 = "http://$(if ($env:GNS3_HOST) { $env:GNS3_HOST } else { '127.0.0.1' }):$(if ($env:GNS3_PORT) { $env:GNS3_PORT } else { 3080 })"
"still on the compute after destroy: " + [bool]((Invoke-RestMethod "$gns3/v2/computes/local/qemu/images") | ForEach-Object { $_ } | Where-Object filename -eq $name)
```

Expect: `md5 of the source: <md5>`. Apply plans `gns3_image.check` with `md5sum` already set to that MD5 and `path` and `filesize` known after apply. It ends `Apply complete! Resources: 1 added, 0 changed, 0 destroyed.` with `listed = "tf-check-<timestamp>.img 46 <md5>"`. `terraform plan` prints `No changes.` Destroy prints the warning `GNS3 image left on the compute` ("GNS3 2.2 has no API to delete images, so \"tf-check-<timestamp>.img\" stays in the qemu images folder on compute \"local\" … It was removed from Terraform state only.") and ends `Destroy complete! Resources: 1 destroyed.` The last line is `still on the compute after destroy: True`. Delete the file from the compute's QEMU images folder by hand if needed (seen 2026-10-15 against GNS3 2.2.61).

Tests: "TestImageResource", "TestImageDataSource", "TestAccImageResource", "TestAccImageDataSource"
