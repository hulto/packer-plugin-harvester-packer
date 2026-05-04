# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0
#
# Example: Build a Harvester VM image using kubeconfig authentication.
# The kubeconfig may use either a bearer token or a client certificate.
#
# Usage (token in kubeconfig):
#   packer build harvester-kubeconfig.pkr.hcl
#
# Usage (override token with a service-account token):
#   packer build -var harvester_token=<sa-token> harvester-kubeconfig.pkr.hcl
#
# The kubeconfig path falls back to $KUBECONFIG or ~/.kube/config when omitted.

variable "kubeconfig" {
  type    = string
  default = ""
  description = "Path to a kubeconfig file. Defaults to $KUBECONFIG or ~/.kube/config."
}

variable "harvester_token" {
  type      = string
  sensitive = true
  default   = ""
  description = "Optional service-account bearer token. Overrides any token embedded in the kubeconfig."
}

variable "namespace" {
  type    = string
  default = "default"
}

source "harvester-clone" "ubuntu" {
  # Use kubeconfig for server URL and TLS settings.
  # An explicit token (if provided) takes priority over any token in the kubeconfig.
  kubeconfig    = var.kubeconfig
  token         = var.harvester_token
  namespace     = var.namespace

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
