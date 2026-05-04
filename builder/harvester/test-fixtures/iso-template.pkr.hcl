# Example Packer template using the Harvester ISO builder.
# Run: PACKER_ACC=1 go test -v ./builder/harvester/ -timeout=120m

source "harvester-iso" "basic-example" {
  harvester_url  = "https://192.168.1.100:6443"
  namespace      = "default"
  token          = "test-token"
  skip_tls_verify = true

  vm_name    = "packer-test-iso"
  cpu_cores  = 2
  memory_mb  = 2048
  disk_size  = "20Gi"

  iso_image_name      = "ubuntu-22-04-iso"
  iso_image_namespace = "default"

  output_image_name      = "packer-ubuntu-22-04"
  output_image_namespace = "default"

  communicator = "ssh"
  ssh_username = "ubuntu"
  ssh_password = "ubuntu"
  ssh_timeout  = "15m"

  boot_wait = "10s"
  boot_command = [
    "<enter>",
    "<wait5>",
    "autoinstall<enter>",
  ]
}

build {
  sources = [
    "source.harvester-iso.basic-example"
  ]
}
