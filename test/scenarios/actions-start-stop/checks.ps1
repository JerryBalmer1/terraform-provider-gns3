# Asserts that the after_create action on the link started both nodes, then invokes the
# gns3_stop_nodes action with terraform apply -invoke and asserts that both nodes stopped.
# GET /v2/projects/{project_id}/nodes/{node_id}: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeid.html
# terraform apply -invoke: https://developer.hashicorp.com/terraform/language/invoke-actions
param(
    [Parameter(Mandatory)] [string] $Server,
    [Parameter(Mandatory)] [hashtable] $Outputs
)
$ErrorActionPreference = 'Stop'

function Assert-Equal($What, $Expected, $Actual) {
    if ("$Expected" -ne "$Actual") {
        throw "${What}: expected '$Expected', server has '$Actual'"
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

function Assert-NodeStatus($ProjectId, $Expected, $When) {
    foreach ($name in 'PC1', 'PC2') {
        $nodeId = $Outputs.nodes.$name
        if (-not $nodeId) { throw "Output nodes has no entry for $name." }
        $node = Invoke-RestMethod -Uri "$Server/v2/projects/$ProjectId/nodes/$nodeId"
        Assert-Equal "$name name" $name $node.name
        Assert-Equal "$name status $When" $Expected $node.status
    }
}

$id = $Outputs.project_id
if (-not $id) { throw 'Output project_id is empty.' }

# (a) apply ran action.gns3_start_nodes.start after creating the link.
Assert-NodeStatus $id 'started' 'after apply (action_trigger after_create on the link)'

# (b) Invoke the stop action. Gns3Range calls checks.ps1 in its own process, so TF_CLI_CONFIG_FILE
# (the dev override under -Dev), GNS3_HOST and GNS3_PORT are already set; -chdir picks this folder
# whatever the current directory is.
$tf = Get-TerraformExe
& $tf "-chdir=$PSScriptRoot" apply '-invoke=action.gns3_stop_nodes.stop' -auto-approve -input=false -no-color | ForEach-Object { Write-Host $_ }
if ($LASTEXITCODE -ne 0) {
    throw "terraform apply -invoke=action.gns3_stop_nodes.stop failed with exit code $LASTEXITCODE"
}

# (c) Both nodes are stopped now.
Assert-NodeStatus $id 'stopped' 'after terraform apply -invoke=action.gns3_stop_nodes.stop'

Write-Host "Project ${id}: PC1 and PC2 were started by the after_create action and stopped by the invoked action."
