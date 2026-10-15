terraform {
  required_providers {
    gns3 = {
      source  = "JerryBalmer1/gns3"
      version = "~> 0.1"
    }
  }
}

# Every argument is optional. Unset arguments fall back to GNS3_HOST,
# GNS3_PORT, GNS3_USERNAME, GNS3_PASSWORD and GNS3_TOKEN, then to the
# defaults (http://127.0.0.1:3080, no auth).
provider "gns3" {
  host = "127.0.0.1"
  port = 3080
}
