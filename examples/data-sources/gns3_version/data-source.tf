data "gns3_version" "server" {}

output "gns3_version" {
  value = data.gns3_version.server.version
}
