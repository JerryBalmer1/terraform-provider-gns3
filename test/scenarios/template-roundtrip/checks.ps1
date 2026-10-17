# Asserts that the gns3_template exists as a user vpcs template and that the one node in the
# project was created from it.
# GET /v2/templates/{template_id}: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/template/templatestemplateid.html
# GET /v2/projects/{project_id}: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectid.html
# GET /v2/projects/{project_id}/nodes: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
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

$templateId = $Outputs.template_id
if (-not $templateId) { throw 'Output template_id is empty.' }
$template = Invoke-RestMethod -Uri "$Server/v2/templates/$templateId"
Assert-Equal 'template name' 'tf-scenario-template-roundtrip' $template.name
Assert-Equal 'template template_type' 'vpcs' $template.template_type
Assert-Equal 'template builtin' 'False' $template.builtin
Assert-Equal 'template compute_id' 'local' $template.compute_id

$id = $Outputs.project_id
if (-not $id) { throw 'Output project_id is empty.' }
$project = Invoke-RestMethod -Uri "$Server/v2/projects/$id"
Assert-Equal 'project name' 'tf-scenario-template-roundtrip' $project.name

$nodes = @(Invoke-RestMethod -Uri "$Server/v2/projects/$id/nodes" | ForEach-Object { $_ })
Assert-Equal 'node count' 1 $nodes.Count
Assert-Equal 'node_id' $Outputs.node_id $nodes[0].node_id
Assert-Equal 'node name' 'PC1' $nodes[0].name
Assert-Equal 'node node_type' 'vpcs' $nodes[0].node_type
Assert-Equal 'node template_id' $templateId $nodes[0].template_id

Write-Host "Template $templateId (vpcs, user) exists, and node PC1 ($($nodes[0].node_id)) in project $id was created from it."
