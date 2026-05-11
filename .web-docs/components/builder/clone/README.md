by cloning an existing `VirtualMachineImage`, booting a VM from the clone,
running provisioners, and exporting the customised disk as a new
`VirtualMachineImage`.

**Requirements**

- A running Harvester cluster (v1.0+) accessible from the machine running Packer.
- A Kubernetes service-account bearer token **or** a kubeconfig file with sufficient
  permissions to create and manage `VirtualMachine`, `VirtualMachineInstance`,
  and `VirtualMachineImage` resources.
- The source image must already exist in Harvester as a `VirtualMachineImage`.

**How it works**

1. A temporary `VirtualMachine` is created with a root disk cloned from the
   specified source image.
2. The VM is started.
3. Packer waits for the VM to become `Running` and connects via SSH (or WinRM).
4. Provisioners run inside the guest to apply customisations.
5. The VM is stopped gracefully.
6. A new `VirtualMachineImage` is exported from the customised root disk.
7. The temporary VM is cleaned up.

<!-- Builder Configuration Fields -->

**Required**

- `harvester_url` (string) - Base URL of the Harvester API server,
  e.g. `https://192.168.1.100:6443`. Required unless `kubeconfig` is set.

- `source_image_name` (string) - Name (or display name) of the source
  `VirtualMachineImage` in Harvester to clone as the root disk.

- `ssh_username` (string) - Username to use for SSH access.

**Optional**

- `kubeconfig` (string) - Path to a kubeconfig file. When set, the server URL,
  CA certificate, and credentials are read from the file. An explicit `token`
  or `harvester_url` in the builder config takes priority over the values in
  the kubeconfig. If the kubeconfig user has no bearer token and provides a
  client certificate + key pair, mTLS authentication is used automatically.
  When `kubeconfig` is empty the plugin falls back to the `$KUBECONFIG`
  environment variable, then `~/.kube/config`.

- `namespace` (string) - Kubernetes namespace for VM resources. Defaults to
  `default`.

- `token` (string) - Kubernetes service-account bearer token for authenticating
  against the Harvester API.

- `skip_tls_verify` (bool) - Disable TLS certificate verification. Not
  recommended for production. Defaults to `false`.

- `vm_name` (string) - Name of the temporary build VM. Auto-generated when
  omitted.

- `cpu_cores` (int) - Number of vCPU cores for the build VM. Defaults to `2`.

- `memory_mb` (int) - RAM for the build VM in megabytes. Defaults to `2048`.

- `disk_size` (string) - Size of the root disk, e.g. `"40Gi"`. Defaults to
  `"40Gi"`.

- `storage_class` (string) - Kubernetes StorageClass for persistent volumes.
  Defaults to `"harvester-longhorn"`.

- `network_name` (string) - Name of the Harvester network attachment or leave
  empty to use the default pod network.

- `network_namespace` (string) - Namespace of the `NetworkAttachmentDefinition`
  when using a Multus network.

- `source_image_namespace` (string) - Namespace of the source
  `VirtualMachineImage`. Defaults to `namespace`.

- `output_image_name` (string) - Display name for the output
  `VirtualMachineImage`. Defaults to `"packer-<vm_name>"`.

- `output_image_namespace` (string) - Namespace for the output image. Defaults
  to `namespace`.

- `wait_for_instance_timeout` (duration string) - Maximum time to wait for the
  VM to reach Running state. Defaults to `"10m"`.

- `shutdown_command` (string) - Shell command to run inside the guest to
  initiate shutdown. When omitted a graceful stop is issued via the API.

- `shutdown_timeout` (duration string) - Time to wait for the VM to stop.
  Defaults to `"5m"`.

All standard `ssh_*` and `winrm_*` communicator fields are also supported.

### Example Usage

```hcl
source "harvester-clone" "ubuntu" {
  harvester_url  = "https://192.168.1.100:6443"
  namespace      = "default"
  token          = var.harvester_token
  skip_tls_verify = true

  cpu_cores = 2
  memory_mb = 4096
  disk_size = "40Gi"

  source_image_name      = "ubuntu-22-04-base"
  source_image_namespace = "default"

  output_image_name = "ubuntu-22-04-nginx"

  communicator = "ssh"
  ssh_username = "ubuntu"
  ssh_private_key_file = "~/.ssh/id_rsa"
  ssh_timeout = "10m"
}

build {
  sources = ["source.harvester-clone.ubuntu"]

  provisioner "shell" {
    inline = [
      "sudo apt-get update",
      "sudo apt-get install -y nginx",
      "sudo systemctl enable nginx",
    ]
  }
}
```
