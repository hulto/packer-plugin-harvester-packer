# Copyright IBM Corp. 2020, 2026
# SPDX-License-Identifier: MPL-2.0
#
# Example: Build a Linux Mint 22.3 (Zena) golden image from ISO in Harvester.
# The ISO must already exist as harvester-public/mint-zena-22.3.iso
#
# The preseed file is served via Packer's built-in HTTP server (http_directory).
# The boot command passes url= pointing at that server to the Debian installer.
#
# Usage:
#   packer build harvester-iso-mint22-golden.pkr.hcl

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

source "harvester-iso" "mint22_golden" {
  # Harvester connection via kubeconfig
  kubeconfig = var.kubeconfig
  namespace  = var.namespace

  # VM resources for installer run
  cpu_cores     = 4
  memory_mb     = 16384
  disk_size     = "40Gi"
  storage_class = "duplicated"

  # Source ISO image (Mint 22.3 Zena)
  iso_image_name      = "mint-zena-22.3.iso"
  iso_image_namespace = "harvester-public"

  # Published output image
  output_image_name      = "mint-zena-22.3"
  output_image_namespace = "harvester-public"

  # SSH communicator — credentials created by preseed (packer / packer)
  communicator = "ssh"
  ssh_username = "packer"
  ssh_password = "packer"
  ssh_timeout  = "90m"

  # Linux Mint 22 boots to a live desktop by default.  The boot_command
  # selects "Install Linux Mint" from the GRUB menu, then appends preseed
  # kernel parameters so the Debian installer runs fully unattended.
  #
  # /dev/sr0  = Mint installer ISO
  # /dev/sr1  = our cidata ISO (cd_files below), label "PRSEED"
  # Ubiquity mounts /dev/sr1 at /media/mint/PRSEED in the live session.
  boot_wait = "10s"
  boot_command = [
    # Move to the "Install Linux Mint" entry (second item in GRUB menu)
    "<down>",
    # Open GRUB edit mode for this entry
    "<tab>",
    " automatic-ubiquity file=/media/mint/PRSEED/preseed.cfg",
    " netcfg/get_hostname=mint-golden quiet",
    "<enter>",
  ]

  # Auxiliary CD-ROM containing only the preseed file, labelled "PRSEED".
  # Packer attaches this as /dev/sr1; udisks mounts it at /media/mint/PRSEED.
  cd_label = "PRSEED"
  cd_files = [
    "${path.root}/http/mint-22/preseed.cfg",
  ]
}

build {
  sources = ["source.harvester-iso.mint22_golden"]

  # Post-install hardening and cloud-init preparation for the golden image.
  # cloud-init is already installed and enabled by the preseed late-commands;
  # we clean its state here so first-boot provisioning runs fresh on cloned VMs.
  provisioner "shell" {
    inline = [
      "set -euxo pipefail",
      "sudo apt-get update -y",
      "sudo apt-get install -y qemu-guest-agent cloud-init",
      "sudo systemctl enable qemu-guest-agent",
      "sudo systemctl enable cloud-init",
      "sudo apt-get autoremove -y",
      "sudo apt-get clean",
      # Reset machine-id so clones each get a unique ID on first boot
      "sudo truncate -s 0 /etc/machine-id",
      # Clear cloud-init state so provisioning reruns on cloned VMs
      "sudo cloud-init clean --logs --seed || true",
    ]
  }
}
