# The image must already be on the compute.
import {
  to = gns3_image.vyos
  identity = {
    compute_id = "local"
    emulator   = "qemu"
    filename   = "vyos-2026.10.07-0712-rolling-generic-amd64.iso"
  }
}
