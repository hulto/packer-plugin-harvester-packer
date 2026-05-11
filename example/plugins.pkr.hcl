# Copyright IBM Corp. 2020, 2026
# SPDX-License-Identifier: MPL-2.0

packer {
  required_plugins {
    harvester = {
      source  = "github.com/hashicorp/scaffolding"
      version = ">= 0.0.1"
    }
  }
}
