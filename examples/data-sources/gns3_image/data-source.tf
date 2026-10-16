# An image already on the compute, uploaded through the GNS3 GUI or copied
# into its images folder by hand.
data "gns3_image" "vyos" {
  emulator = "qemu"
  filename = "vyos-2026.10.07-0712-rolling-generic-amd64.iso"
}

output "vyos_image_md5" {
  value = data.gns3_image.vyos.md5sum
}
