# Runs terraform query against adopt.tfquery.hcl and asserts that list.gns3_project.all finds the
# project from main.tf and that list.gns3_node.in_project finds no nodes in it.
# GET /v2/projects/{project_id}: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectid.html
# GET /v2/projects/{project_id}/nodes: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
# terraform query: https://developer.hashicorp.com/terraform/cli/commands/query
# -json messages (list_start, list_resource_found, list_complete): https://developer.hashicorp.com/terraform/internals/machine-readable-ui
param(
    [Parameter(Mandatory)] [string] $Server,
    [Parameter(Mandatory)] [hashtable] $Outputs
)
$ErrorActionPreference = 'Stop'
# Invoke-RestMethod writes a JSON array to the pipeline as one object; ForEach-Object unrolls it.

function Assert-Equal($What, $Expected, $Actual) {
    if ("$Expected" -ne "$Actual") {
        throw "${What}: expected '$Expected', got '$Actual'"
    }
}

function Get-TerraformExe {
    # $env:TERRAFORM_EXE, then the per-user install, then PATH.
    if ($env:TERRAFORM_EXE) {
        if (-not (Test-Path -LiteralPath $env:TERRAFORM_EXE)) { throw "TERRAFORM_EXE is '$env:TERRAFORM_EXE', which does not exist." }
        return $env:TERRAFORM_EXE
    }
    if ($env:LOCALAPPDATA) {
        $local = Join-Path $env:LOCALAPPDATA 'Programs' 'Terraform' 'terraform.exe'
        if (Test-Path -LiteralPath $local) { return $local }
    }
    $cmd = Get-Command terraform -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $cmd) { throw 'terraform not found: set TERRAFORM_EXE, or put terraform on PATH.' }
    $cmd.Source
}

$id = $Outputs.project_id
if (-not $id) { throw 'Output project_id is empty.' }

# The server side first, so a failure below is clearly about terraform query.
$project = Invoke-RestMethod -Uri "$Server/v2/projects/$id"
Assert-Equal 'project name' 'tf-scenario-query-adopt' $project.name
$serverNodes = @(Invoke-RestMethod -Uri "$Server/v2/projects/$id/nodes" | ForEach-Object { $_ })
Assert-Equal 'nodes on the server' 0 $serverNodes.Count

# Gns3Range calls checks.ps1 in its own process, so TF_CLI_CONFIG_FILE (the dev override under
# -Dev), GNS3_HOST and GNS3_PORT are already set; -chdir picks this folder whatever the current
# directory is.
$tf = Get-TerraformExe
$lines = @(& $tf "-chdir=$PSScriptRoot" query -json -no-color -var "project_id=$id")
$exit = $LASTEXITCODE
$messages = @(foreach ($line in $lines) {
        $text = "$line".Trim()
        if ($text.StartsWith('{')) { $text | ConvertFrom-Json }
    })
$errors = @($messages | Where-Object { $_.'@level' -eq 'error' } | ForEach-Object { $_.'@message' })
if ($exit -ne 0) {
    throw "terraform query -json failed with exit code ${exit}: $(if ($errors) { $errors -join '; ' } else { $lines -join "`n" })"
}
if ($errors) { throw "terraform query -json reported errors: $($errors -join '; ')" }

function Get-Found($Address) {
    @($messages | Where-Object { $_.type -eq 'list_resource_found' -and $_.list_resource_found.address -eq $Address } |
            ForEach-Object { $_.list_resource_found })
}

function Get-Total($Address) {
    $done = @($messages | Where-Object { $_.type -eq 'list_complete' -and $_.list_complete.address -eq $Address })
    if ($done.Count -ne 1) { throw "terraform query -json: expected one list_complete for $Address, got $($done.Count)." }
    $done[0].list_complete.total
}

# list.gns3_project.all: one result whose identity carries the applied project's project_id.
$projects = Get-Found 'list.gns3_project.all'
$ids = @($projects | ForEach-Object { if ($_.identity) { $_.identity.PSObject.Properties.Value } })
$matched = @($projects | Where-Object { $_.identity -and $_.identity.PSObject.Properties.Value -contains $id })
if ($matched.Count -ne 1) {
    throw "list.gns3_project.all: expected exactly one result whose identity holds project_id '$id', got $($matched.Count). Identity values found: $($ids -join ', ')"
}
Assert-Equal 'list.gns3_project.all resource_type' 'gns3_project' $matched[0].resource_type
Assert-Equal 'list.gns3_project.all total (list_complete) vs results' $projects.Count (Get-Total 'list.gns3_project.all')

# list.gns3_node.in_project: main.tf creates no nodes, so nothing is found.
$nodes = Get-Found 'list.gns3_node.in_project'
Assert-Equal 'list.gns3_node.in_project results' 0 $nodes.Count
Assert-Equal 'list.gns3_node.in_project total (list_complete)' 0 (Get-Total 'list.gns3_node.in_project')

Write-Host "terraform query found project $id among $($projects.Count) project(s) and no nodes in it."
