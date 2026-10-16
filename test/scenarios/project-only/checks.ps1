# Asserts that the project from main.tf exists on the server with the settings main.tf gave it.
# GET /v2/projects/{project_id}: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectid.html
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
Assert-Equal 'project name' 'tf-scenario-project-only' $project.name
Assert-Equal 'project name (output)' $Outputs.name $project.name
Assert-Equal 'project filename' $Outputs.filename $project.filename
Assert-Equal 'grid_size' 50 $project.grid_size
Assert-Equal 'scene_width' 3000 $project.scene_width
Assert-Equal 'scene_height' 1500 $project.scene_height
Assert-Equal 'show_grid' $true $project.show_grid

$listed = @(Invoke-RestMethod -Uri "$Server/v2/projects" | ForEach-Object { $_ } | Where-Object project_id -eq $id)
Assert-Equal 'projects listed with this project_id' 1 $listed.Count

Write-Host "Project $id '$($project.name)' has the expected settings."
