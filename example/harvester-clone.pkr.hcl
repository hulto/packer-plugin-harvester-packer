# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0
#
# Example: Build a Harvester VM image by cloning an existing base image.
# Requires: packer-plugin-harvester installed and a Harvester cluster.
#
# Usage:
#   packer build -var harvester_token=<token> harvester-clone.pkr.hcl

variable "harvester_url" {
  type    = string
  default = "https://192.168.1.100:6443"
}

variable "harvester_token" {
  type      = string
  sensitive = true
}

variable "namespace" {
  type    = string
  default = "default"
}

source "harvester-clone" "ubuntu" {
  # Harvester connection
  harvester_url   = var.harvester_url
  namespace       = var.namespace
  token           = var.harvester_token
  skip_tls_verify = true

  # VM resources
  cpu_cores = 2
  memory_mb = 4096
  disk_size = "40Gi"

  # Source image to clone (must already exist in Harvester)
  source_image_name      = "ubuntu-22-04-base"
  source_image_namespace = var.namespace

  # Output image
  output_image_name      = "packer-ubuntu-22-04-nginx"
  output_image_namespace = var.namespace

  # SSH communicator
  communicator         = "ssh"
  ssh_username         = "ubuntu"
  ssh_private_key_file = "~/.ssh/id_rsa"
  ssh_timeout          = "10m"
}

build {
  sources = ["source.harvester-clone.ubuntu"]

  provisioner "shell" {
    inline = [
      "echo 'Installing nginx...'",
      "sudo apt-get update -y",
      "sudo apt-get install -y nginx",
      "sudo systemctl enable nginx",
    ]
  }
}
