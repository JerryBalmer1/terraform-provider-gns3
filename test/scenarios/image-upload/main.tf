terraform {
  required_providers {
    gns3 = {
      source  = "JerryBalmer1/gns3"
      version = "0.2.0"
    }
  }
}

# Empty: the provider defaults to http://127.0.0.1:3080 (GNS3_HOST and
# GNS3_PORT override it).
provider "gns3" {}

# Written by Invoke-Build Gns3Range before init (same bytes every run), because
# this scenario requires "image". Not a bootable disk.
locals {
  source = abspath("${path.module}/../../images/scenario-dummy.img")
}

resource "gns3_image" "dummy" {
  compute_id  = "local"
  emulator    = "qemu"
  filename    = "tf-scenario-dummy.img"
  source_path = local.source
  md5sum      = filemd5(local.source)
}

output "source_path" {
  value = local.source
}

output "filename" {
  value = gns3_image.dummy.filename
}

output "md5sum" {
  value = gns3_image.dummy.md5sum
}

output "filesize" {
  value = gns3_image.dummy.filesize
}

output "path" {
  value = gns3_image.dummy.path
}
