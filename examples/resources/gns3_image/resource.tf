# VyOS rolling release ISO (about 612 MiB) from the official nightly builds.
# Rolling builds are removed after a few months: pick a current one at
# https://vyos.net/get/nightly-builds/ and update both names below.
resource "gns3_image" "vyos" {
  compute_id = "local"
  emulator   = "qemu"
  filename   = "vyos-2026.10.07-0712-rolling-generic-amd64.iso"
  source_url = "https://github.com/vyos/vyos-nightly-build/releases/download/2026.10.07-0712-rolling/vyos-2026.10.07-0712-rolling-generic-amd64.iso"

  # VyOS signs its builds with minisign rather than publishing an MD5. After
  # checking the signature, set md5sum to pin the content:
  # (Get-FileHash <iso> -Algorithm MD5).Hash.ToLower()
  # md5sum = "..."
}

# MikroTik CHR from a file on the machine running Terraform (unzip
# chr-7.24.5.img.zip from https://mikrotik.com/download first). With md5sum
# taken from the file itself, replacing the local file uploads it again.
resource "gns3_image" "chr" {
  emulator    = "qemu"
  filename    = "chr-7.24.5.img"
  source_path = "${path.module}/images/chr-7.24.5.img"
  md5sum      = filemd5("${path.module}/images/chr-7.24.5.img")
}

# Destroying either resource leaves the file on the compute: GNS3 2.2 has no
# API to delete images.
