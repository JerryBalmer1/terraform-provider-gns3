# Asserts that each VPCS node's startup.vpc on the server holds the content of its file beside
# main.tf (pc1.vpc, pc2.vpc), with the expected address, and that the two nodes are linked.
# GET /v2/projects/{project_id}/nodes: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
# GET /v2/projects/{project_id}/nodes/{node_id}/files/{path}: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesnodeidfilespath.html
#   (2.2.61 answers 200 with the raw file as application/octet-stream; the docs list only 204, see docs/api-drift.md)
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
Assert-Equal 'project name' 'tf-scenario-vpcs-startup-config' $project.name

$nodes = @(Invoke-RestMethod -Uri "$Server/v2/projects/$id/nodes" | ForEach-Object { $_ })
Assert-Equal 'node count' 2 $nodes.Count

$expected = @{
    PC1 = @{ File = 'pc1.vpc'; Ip = 'ip 10.0.0.1/24' }
    PC2 = @{ File = 'pc2.vpc'; Ip = 'ip 10.0.0.2/24' }
}
foreach ($name in 'PC1', 'PC2') {
    $want = $Outputs.nodes.$name
    if (-not $want) { throw "Output nodes has no entry for $name." }
    $node = @($nodes | Where-Object name -eq $name)
    Assert-Equal "nodes named $name" 1 $node.Count
    Assert-Equal "$name node_id" $want.node_id $node[0].node_id
    Assert-Equal "$name node_type" 'vpcs' $node[0].node_type

    $response = Invoke-WebRequest -Uri "$Server/v2/projects/$id/nodes/$($want.node_id)/files/startup.vpc"
    $content = $response.Content
    if ($content -is [byte[]]) { $content = [Text.Encoding]::UTF8.GetString($content) }
    $local = [IO.File]::ReadAllText((Join-Path $PSScriptRoot $expected[$name].File))
    if ($content -cne $local) {
        throw "$name startup.vpc: expected the content of $($expected[$name].File):`n$local`nserver has:`n$content"
    }
    $lines = $content -split "`r?`n"
    if ($lines -cnotcontains $expected[$name].Ip) {
        throw "$name startup.vpc has no line '$($expected[$name].Ip)'. Content:`n$content"
    }
}

$links = @(Invoke-RestMethod -Uri "$Server/v2/projects/$id/links" | ForEach-Object { $_ })
Assert-Equal 'link count' 1 $links.Count
Assert-Equal 'link_id' $Outputs.link_id $links[0].link_id
foreach ($name in 'PC1', 'PC2') {
    $end = @($links[0].nodes | Where-Object node_id -eq $Outputs.nodes.$name.node_id)
    Assert-Equal "link ends on $name" 1 $end.Count
}

Write-Host "Project $id has PC1 (10.0.0.1/24) and PC2 (10.0.0.2/24) with startup.vpc as written, linked by $($links[0].link_id)."
