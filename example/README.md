## The Example Folder

This folder must contain a fully working example of the plugin usage. The example must define the `required_plugins`
block. A pre-defined GitHub Action will run `packer init`, `packer validate`, and `packer build` to test your plugin 
with the latest version available of Packer.

The folder can contain multiple HCL2 compatible files. The action will execute Packer at this folder level
running `packer init -upgrade .` and `packer build .`.

If the plugin requires authentication, the configuration should be provided via GitHub Secrets and set as environment
variables in the [test-plugin-example.yml](/.github/workflows/test-plugin-example.yml) file. Example:

```yml
  - name: Build
    working-directory: ${{ github.event.inputs.folder }}
    run: PACKER_LOG=${{ github.event.inputs.logs }} packer build .
    env:
      AUTH_KEY: ${{ secrets.AUTH_KEY }}
      AUTH_PASSWORD: ${{ secrets.AUTH_PASSWORD }}
```

## Harvester Ubuntu 24 ISO Golden Image Example

Use `harvester-iso-ubuntu24-golden.pkr.hcl` to build a VM from an Ubuntu 24 ISO and publish it as a
`VirtualMachineImage`.

This example is preconfigured for:

- Source ISO image: `harvester-public/image-57448`
- Build namespace: `hulto`
- Output image: `harvester-public/ubuntu-24-golden`
- Kubeconfig path: `~/.kube/config`

Before running the example, rebuild and reinstall the plugin from the repository root:

```sh
cd ..
make dev
```

If you do not use `make`, run the equivalent commands manually:

```sh
go build -ldflags="-X github.com/hashicorp/packer-plugin-scaffolding/version.Version=0.2.1 -X github.com/hashicorp/packer-plugin-scaffolding/version.VersionPrerelease=dev" -o packer-plugin-scaffolding
packer plugins install --path packer-plugin-scaffolding github.com/hashicorp/scaffolding
cd example/

# Mint 22
packer init harvester-iso-mint22-golden.pkr.hcl
packer build -on-error=ask -var "kubeconfig=/persistent/workspaces/.kube/config" harvester-iso-mint22-golden.pkr.hcl


# ubuntu 24
packer init harvester-iso-ubuntu24-golden.pkr.hcl
packer build -on-error=abort -var "kubeconfig=/persistent/workspaces/.kube/config" harvester-iso-ubuntu24-golden.pkr.hcl
```
