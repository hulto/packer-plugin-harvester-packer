# Copyright IBM Corp. 2020, 2026
# SPDX-License-Identifier: MPL-2.0
#
# Example: Build an Ubuntu 24 golden image from ISO in Harvester.
# The ISO image must already exist as:
#   harvester-public/image-57448
#
# Usage:
#   packer build harvester-iso-ubuntu24-golden.pkr.hcl

packer {
  required_plugins {
    harvester = {
      source  = "github.com/hashicorp/scaffolding"
      version = ">= 0.2.0"
    }
  }
}

variable "kubeconfig" {
  type    = string
  default = "~/.kube/config"
}

variable "namespace" {
  type    = string
  default = "hulto"
}

source "harvester-iso" "ubuntu24_golden" {
  # Harvester connection via kubeconfig
  kubeconfig = var.kubeconfig
  namespace  = var.namespace

  # VM resources for installer run
  cpu_cores = 2
  memory_mb = 4096
  disk_size = "40Gi"
  storage_class = "duplicated"

  # Source ISO image
  iso_image_name      = "image-57448"
  iso_image_namespace = "harvester-public"

  # Published output image
  output_image_name      = "ubuntu-24-golden"
  output_image_namespace = "harvester-public"

  # SSH communicator (credentials created by autoinstall user-data)
  communicator = "ssh"
  ssh_username = "ubuntu"
  ssh_password = "ubuntu"
  ssh_timeout  = "30m"

  # Ubuntu 24 Server autoinstall over NoCloud HTTP
  boot_wait = "10s"
  boot_command = [
    "<enter>",
    "<wait><wait><wait>",
    "e",
    "<down><down><down><end>",
    " autoinstall ds=nocloud-net\\;s=http://{{.HTTPIP}}:{{.HTTPPort}}/",
    "<F10>",
  ]

  http_directory = "./http/ubuntu-24"
}

build {
  sources = ["source.harvester-iso.ubuntu24_golden"]

  # Apply baseline packages and hardening-ready defaults for the golden image.
  provisioner "shell" {
    inline = [
      "set -euxo pipefail",
      "sudo apt-get update -y",
      "sudo apt-get install -y qemu-guest-agent cloud-init",
      "sudo systemctl enable qemu-guest-agent",
      "sudo apt-get autoremove -y",
      "sudo apt-get clean",
      "sudo truncate -s 0 /etc/machine-id",
      "sudo cloud-init clean --logs --seed || true",
    ]
  }
}