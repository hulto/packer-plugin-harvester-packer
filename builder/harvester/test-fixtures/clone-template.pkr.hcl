# Example Packer template using the Harvester clone builder.
# Run: PACKER_ACC=1 go test -v ./builder/harvester/ -timeout=120m

source "scaffolding-clone" "basic-example" {
  harvester_url  = "https://192.168.1.100:6443"
  namespace      = "default"
  token          = "test-token"
  skip_tls_verify = true

  vm_name    = "packer-test-clone"
  cpu_cores  = 2
  memory_mb  = 2048
  disk_size  = "20Gi"

  source_image_name      = "ubuntu-22-04"
  source_image_namespace = "default"

  output_image_name      = "packer-ubuntu-22-04-custom"
  output_image_namespace = "default"

  communicator = "ssh"
  ssh_username = "ubuntu"
  ssh_password = "ubuntu"
  ssh_timeout  = "15m"
}

build {
  sources = [
    "source.scaffolding-clone.basic-example"
  ]

  provisioner "shell-local" {
    inline = ["echo Build complete"]
  }
}
