# Asserts that the project from main.tf holds exactly the two VPCS nodes and the one link
# between their first ports.
# GET /v2/projects/{project_id}/nodes: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
# GET /v2/projects/{project_id}/links: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/link/projectsprojectidlinks.html
param(
    [Parameter(Mandatory)] [string] $Server,
    [Parameter(Mandatory)] [hashtable] $Outputs
)
$ErrorActionPreference = 'Stop'
# Invoke-RestMethod writes a JSON array to the pipeline as one object; ForEach-Object unrolls it.

function Assert-Equal($What, $Expected, $Actual) {
    if ("$Expected" -ne "$Actual") {
        throw "${What}: expected '$Expected', server has '$Actual'"
    }
}

$id = $Outputs.project_id
if (-not $id) { throw 'Output project_id is empty.' }
$project = Invoke-RestMethod -Uri "$Server/v2/projects/$id"
Assert-Equal 'project name' 'tf-scenario-two-nodes-one-link' $project.name

$nodes = @(Invoke-RestMethod -Uri "$Server/v2/projects/$id/nodes" | ForEach-Object { $_ })
Assert-Equal 'node count' 2 $nodes.Count
foreach ($name in 'PC1', 'PC2') {
    $want = $Outputs.nodes.$name
    if (-not $want) { throw "Output nodes has no entry for $name." }
    $node = @($nodes | Where-Object name -eq $name)
    Assert-Equal "nodes named $name" 1 $node.Count
    $node = $node[0]
    Assert-Equal "$name node_id" $want.node_id $node.node_id
    Assert-Equal "$name node_type" 'vpcs' $node.node_type
    Assert-Equal "$name status (start_on_create unset)" 'stopped' $node.status
}

$links = @(Invoke-RestMethod -Uri "$Server/v2/projects/$id/links" | ForEach-Object { $_ })
Assert-Equal 'link count' 1 $links.Count
$link = $links[0]
Assert-Equal 'link_id' $Outputs.link_id $link.link_id
Assert-Equal 'link_type' 'ethernet' $link.link_type
Assert-Equal 'link ends' 2 @($link.nodes).Count
foreach ($name in 'PC1', 'PC2') {
    $want = $Outputs.nodes.$name
    $end = @($link.nodes | Where-Object node_id -eq $want.node_id)
    Assert-Equal "link ends on $name" 1 $end.Count
    Assert-Equal "$name link adapter_number" $want.adapter_number $end[0].adapter_number
    Assert-Equal "$name link port_number" $want.port_number $end[0].port_number
}

Write-Host "Project $id has PC1 and PC2 (vpcs, stopped) and link $($link.link_id) between them."
