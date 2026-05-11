<!--
  Include a short overview about the plugin.

  This document is a great location for creating a table of contents for each
  of the components the plugin may provide. This document should load automatically
  when navigating to the docs directory for a plugin.

-->

### Installation

To install this plugin, copy and paste this code into your Packer configuration, then run [`packer init`](https://www.packer.io/docs/commands/init).

```hcl
packer {
  required_plugins {
    harvester = {
      # source represents the GitHub URI to the plugin repository without the `packer-plugin-` prefix.
      source  = "github.com/hulto/harvester"
      version = ">=0.0.1"
    }
  }
}
```

Alternatively, you can use `packer plugins install` to manage installation of this plugin.

```sh
$ packer plugins install github.com/hulto/harvester
```

### Components

The Harvester plugin provides builders for creating and cloning VM images on Harvester.

#### Builders

- [clone](/packer/integrations/hulto/harvester/latest/components/builder/clone) - Creates a new image by cloning an existing Harvester VM template.
- [iso](/packer/integrations/hulto/harvester/latest/components/builder/iso) - Installs an image from ISO media and exports it as a Harvester image.

#### Provisioners

- [provisioner](/packer/integrations/hulto/harvester/latest/components/provisioner/provisioner) - Template provisioner component for Packer builds.

#### Post-processors

- [post-processor](/packer/integrations/hulto/harvester/latest/components/post-processor/post-processor) - Template post-processor component.

#### Data Sources

- [data source](/packer/integrations/hulto/harvester/latest/components/data-source/datasource) - Template data source component.

