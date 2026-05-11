# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# Details on integration metadata can be found at https://developer.hashicorp.com/packer/docs/plugins/creation
# This metadata.hcl file and the adjacent `components` docs directory should
# be kept in a `.web-docs` directory at the root of your plugin repository.
integration {
  name = "Harvester"
  description = "Build and clone virtual machine images in Harvester with Packer"
  identifier = "packer/hashicorp/harvester"
  flags = [
    # Remove if the plugin does not conform to the HCP Packer requirements.
    #
    # Please refer to our docs if you want your plugin to be compatible with
    # HCP Packer: https://developer.hashicorp.com/packer/docs/plugins/creation/hcp-support
    "hcp-ready",
    # This signals that the plugin is unmaintained and will eventually not be
    # working with a future version of Packer.
    #
    # On the integrations, this will end-up as an icon on the plugin's main card.
    "archived",
  ]
  docs {
    # If you'd prefer not to publish docs on HashiCorp websites, you can
    # set `process_docs` to `false`. If `process_docs` is `false`, you MUST
    # provide a `external_url` so we can link back to your plugin repo.
    process_docs = true
    # Note that the README location is relative to this file. We recommend
    # keeping the default value, as the adjacent `compile-to-webdocs` script
    # will automatically copy the README from the `docs` directory of this
    # repository to the correct location.
    readme_location = "./README.md"
    # `external_url` allows us to link back to your plugin repo.
    external_url = "https://github.com/hulto/packer-plugin-harvester"
  }
  license {
    type = "MPL-2.0"
    url = "https://github.com/hulto/packer-plugin-harvester/blob/main/LICENSE"
  }
  component {
    type = "builder"
    name = "Harvester Clone Builder"
    slug = "clone"
  }
  component {
    type = "builder"
    name = "Harvester ISO Builder"
    slug = "iso"
  }
  component {
    type = "provisioner"
    name = "Template Provisioner"
    slug = "provisioner"
  }
  component {
    type = "post-processor"
    name = "Template Post-Processor"
    slug = "post-processor"
  }
  component {
    type = "data-source"
    name = "Template Data Source"
    slug = "datasource"
  }
}
