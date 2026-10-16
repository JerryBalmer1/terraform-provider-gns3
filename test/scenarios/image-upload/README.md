# image-upload

Up: [Scenario tests](../README.md)

See also: [project-only](../project-only/README.md), [two-nodes-one-link](../two-nodes-one-link/README.md), [vpcs-startup-config](../vpcs-startup-config/README.md)

Uploads `test/images/scenario-dummy.img` to the `local` compute's qemu images folder as `tf-scenario-dummy.img`, with `gns3_image` (`source_path`, and `md5sum = filemd5(...)` to pin the content).

`Invoke-Build Gns3Range` writes the source file before `terraform init`, because `scenario.json` lists `"image"` in `requires`. It is 1 MiB: a text header followed by a fixed byte pattern, identical on every run and every machine. It is not a bootable disk. Nothing is downloaded. The file is gitignored; only `test/images/.gitkeep` is tracked.

## What checks.ps1 verifies

- The `md5sum` and `filesize` outputs match the MD5 and length of the source file on disk.
- `GET /v2/computes/local/qemu/images` lists exactly one `tf-scenario-dummy.img`, with `path` equal to the file name (top of the `QEMU` folder, not a subfolder), and the same MD5 and size as the source file.

## Destroy leaves the file

GNS3 2.2 has no API to delete images, so `destroy` removes the resource from state and leaves `tf-scenario-dummy.img` on the compute (see `docs/resources/image.md`). That is why the bytes never change: on the next run the provider finds the same MD5 already there and adopts the file without uploading. If the generator ever changes, the next apply fails ("a different file with that name"). Then delete the old file from the compute's `images\QEMU` folder by hand. For a server started by `Invoke-Build Gns3Start`, that folder is `$env:TEMP\gns3\data\images\QEMU`.

## In the GNS3 GUI

No project is created. In **Edit > Preferences > QEMU > Qemu VMs > New** (choose "Run this QEMU VM on the local computer", then the existing-image step), `tf-scenario-dummy.img` appears in the image list with a size of 1.0 MB. It stays there after `destroy`. Don't build a VM from it: it won't boot.
