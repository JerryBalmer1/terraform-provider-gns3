# Asserts that the local compute lists the uploaded file in its qemu images, with the MD5 and
# size of the generated source file.
# GET /v2/computes/{compute_id}/{emulator}/images: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/compute/sidemulatorimages.html
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

$source = $Outputs.source_path
if (-not (Test-Path -LiteralPath $source)) { throw "Source file $source does not exist." }
$md5 = (Get-FileHash -LiteralPath $source -Algorithm MD5).Hash.ToLowerInvariant()
$size = (Get-Item -LiteralPath $source).Length

Assert-Equal 'filename (output)' 'tf-scenario-dummy.img' $Outputs.filename
Assert-Equal 'md5sum (output vs source file)' $md5 $Outputs.md5sum
Assert-Equal 'filesize (output vs source file)' $size $Outputs.filesize

$images = @(Invoke-RestMethod -Uri "$Server/v2/computes/local/qemu/images" -TimeoutSec 60 | ForEach-Object { $_ })
$image = @($images | Where-Object filename -eq 'tf-scenario-dummy.img')
Assert-Equal 'qemu images named tf-scenario-dummy.img' 1 $image.Count
$image = $image[0]
Assert-Equal 'path on the compute (top of the QEMU folder)' 'tf-scenario-dummy.img' $image.path
Assert-Equal 'path (output)' $Outputs.path $image.path
Assert-Equal 'md5sum on the compute' $md5 $image.md5sum
Assert-Equal 'filesize on the compute' $size $image.filesize

Write-Host "Compute local lists qemu image $($image.filename) ($($image.filesize) bytes, md5 $($image.md5sum))."
