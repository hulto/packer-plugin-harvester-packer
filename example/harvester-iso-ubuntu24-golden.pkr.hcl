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
  cpu_cores     = 4
  memory_mb     = 16384
  disk_size     = "40Gi"
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
  ssh_timeout  = "90m"

  # Ubuntu 24 Server autoinstall via the auxiliary NoCloud ISO on /dev/sr1.
  # /dev/sr0 is the Ubuntu installer ISO; /dev/sr1 is our cidata ISO (cd_files).
  # Use ds=nocloud without a path: subiquity scans all block devices for a
  # volume labelled "cidata" and reads user-data/meta-data from it directly,
  # so /dev/sr1 does not need to be pre-mounted.
  boot_wait = "5s"
  boot_command = [
    "<wait5>",
    "e",
    "<down><down><down><end>",
    " autoinstall ds=nocloud",
    "<F10>",
  ]

  # Build and attach an auxiliary NoCloud ISO with cloud-init seed files.
  cd_label = "cidata"
  cd_files = [
    "${path.root}/http/ubuntu-24/meta-data",
    "${path.root}/http/ubuntu-24/user-data",
  ]

  #   cloud_init_user_data = file("${path.root}/http/ubuntu-24/user-data")
  #   cloud_init_meta_data = file("${path.root}/http/ubuntu-24/meta-data")
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