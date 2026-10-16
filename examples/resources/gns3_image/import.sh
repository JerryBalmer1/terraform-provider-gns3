# Import by <compute_id>/<emulator>/<filename>. The image must already be on
# the compute. Set source_url or source_path in the configuration afterwards;
# the first plan then shows it as an in-place change that uploads nothing.
terraform import gns3_image.vyos local/qemu/vyos-2026.10.07-0712-rolling-generic-amd64.iso
