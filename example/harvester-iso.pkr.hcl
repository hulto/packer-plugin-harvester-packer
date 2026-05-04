# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0
#
# Example: Build a Harvester VM image using the ISO builder.
# Requires: packer-plugin-harvester installed and a Harvester cluster.
#
# Usage:
#   packer build -var harvester_token=<token> harvester-iso.pkr.hcl

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

source "scaffolding-iso" "ubuntu" {
  # Harvester connection
  harvester_url   = var.harvester_url
  namespace       = var.namespace
  token           = var.harvester_token
  skip_tls_verify = true

  # VM resources
  cpu_cores = 2
  memory_mb = 4096
  disk_size = "40Gi"

  # ISO image (must already exist in Harvester)
  iso_image_name      = "ubuntu-22-04-live-server"
  iso_image_namespace = var.namespace

  # Output image
  output_image_name      = "packer-ubuntu-22-04"
  output_image_namespace = var.namespace

  # SSH communicator
  communicator = "ssh"
  ssh_username = "ubuntu"
  ssh_password = "ubuntu"
  ssh_timeout  = "20m"

  # Boot commands for Ubuntu Server autoinstall
  boot_wait = "5s"
  boot_command = [
    "<enter>",
    "<wait3>",
    "e",
    "<down><down><down><end>",
    " autoinstall ds=nocloud-net\\;s=http://{{.HTTPIP}}:{{.HTTPPort}}/",
    "<F10>",
  ]

  # HTTP server for preseed/autoinstall user-data
  http_directory = "./http"
}

build {
  sources = ["source.scaffolding-iso.ubuntu"]

  provisioner "shell" {
    inline = [
      "echo 'Applying customizations...'",
      "sudo apt-get update -y",
      "sudo apt-get install -y curl wget",
    ]
  }
}
