# Copyright IBM Corp. 2020, 2026
# SPDX-License-Identifier: MPL-2.0

variable "harvester_url" {
  type    = string
  default = "https://192.168.1.100:6443"
}

variable "harvester_token" {
  type        = string
  sensitive   = true
  default     = ""
  description = "Optional service-account bearer token. Required for URL/token auth, optional with kubeconfig auth."
}

variable "namespace" {
  type    = string
  default = "default"
}

variable "kubeconfig" {
  type        = string
  default     = ""
  description = "Path to a kubeconfig file. Defaults to $KUBECONFIG or ~/.kube/config when empty."
}