by booting a new VM from an installation ISO, running provisioners, and exporting
the root disk as a `VirtualMachineImage`.

**Requirements**

- A running Harvester cluster (v1.0+) accessible from the machine running Packer.
- A Kubernetes service-account bearer token **or** a kubeconfig file with sufficient
  permissions to create `VirtualMachine`, `VirtualMachineInstance`, and
  `VirtualMachineImage` resources.
- The installation ISO must already be uploaded to Harvester as a
  `VirtualMachineImage` resource (with `sourceType: download` or `upload`).

**How it works**

1. A temporary `VirtualMachine` is created with a blank root disk and the ISO
   image attached as a CDROM.
2. If `cd_files` or `cd_content` is configured, Packer builds an auxiliary ISO,
  imports it as a temporary Harvester image, and attaches it as an additional
  CDROM.
3. The VM is started.
4. Optional `boot_command` entries are sent to the VM's VNC console to automate
   the installer.
5. Packer waits for the VM to become `Running` and connects via SSH (or WinRM).
6. Provisioners run inside the guest.
7. The VM is stopped gracefully.
8. A new `VirtualMachineImage` is exported from the root disk.
9. Temporary VM and auxiliary CD image resources are cleaned up.

<!-- Builder Configuration Fields -->

**Required**

- `harvester_url` (string) - Base URL of the Harvester API server,
  e.g. `https://192.168.1.100:6443`. Required unless `kubeconfig` is set.

- `iso_image_name` (string) - Name (or display name) of the `VirtualMachineImage`
  in Harvester that contains the installation ISO.

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

- `iso_image_namespace` (string) - Namespace of the ISO `VirtualMachineImage`.
  Defaults to `namespace`.

- `output_image_name` (string) - Display name for the output
  `VirtualMachineImage`. Defaults to `"packer-<vm_name>"`.

- `output_image_namespace` (string) - Namespace for the output image. Defaults
  to `namespace`.

- `boot_command` (list of strings) - Commands to send to the VM via VNC
  immediately after boot. Supports special tokens like `<enter>`, `<wait>`,
  `<f1>`–`<f12>`, `<leftCtrl>`, etc.

- `boot_wait` (duration string) - How long to wait after the VM starts before
  sending boot commands. Defaults to `"10s"`.

- `http_directory` (string) - Directory served over HTTP during the build for
  preseed/kickstart files. Use `{{.HTTPIP}}` and `{{.HTTPPort}}` in
  `boot_command`.

- `cd_files` (list of strings) - Files and/or directories to include in an
  auxiliary ISO. Directories are copied recursively and globs are supported.
  The generated ISO is imported into Harvester and attached as an additional
  CDROM.

- `cd_content` (map of strings) - Inline path/content pairs to include in the
  auxiliary ISO. Keys are file paths inside the ISO and values are file
  contents. Entries override matching paths from `cd_files`.

- `cd_label` (string) - Volume label for the generated auxiliary ISO.

- `http_port_min` / `http_port_max` (int) - Port range for the local HTTP server.
  Defaults to `8000`–`9000`.

- `cloud_init_user_data` (string) - Cloud-init user-data content to attach as
  a NoCloud drive. When set, the builder skips HTTP delivery and the guest reads
  answer data from the attached NoCloud media.

- `cloud_init_meta_data` (string) - Optional cloud-init meta-data content for
  the NoCloud drive. When omitted, defaults are generated.

- `cloud_init_network_data` (string) - Optional cloud-init network config
  content for the NoCloud drive.

When using `cloud_init_*` fields, `http_directory` and `{{.HTTPIP}}`/
`{{.HTTPPort}}` boot-command templates must not be used in the same source
definition.

- `wait_for_instance_timeout` (duration string) - Maximum time to wait for the
  VM to reach Running state. Defaults to `"10m"`.

- `shutdown_command` (string) - Shell command to run inside the guest to
  initiate shutdown. When omitted a graceful stop is issued via the API.

- `shutdown_timeout` (duration string) - Time to wait for the VM to stop.
  Defaults to `"5m"`.

All standard `ssh_*` and `winrm_*` communicator fields are also supported.

### Example Usage

```hcl
source "harvester-iso" "ubuntu" {
  harvester_url  = "https://192.168.1.100:6443"
  namespace      = "default"
  token          = var.harvester_token
  skip_tls_verify = true

  cpu_cores = 2
  memory_mb = 4096
  disk_size = "40Gi"

  iso_image_name      = "ubuntu-22-04-server"
  iso_image_namespace = "default"

  output_image_name = "ubuntu-22-04-custom"

  communicator = "ssh"
  ssh_username = "ubuntu"
  ssh_password = "ubuntu"
  ssh_timeout  = "20m"

  boot_wait = "5s"
  boot_command = [
    "<enter><enter>",
    "<wait5>",
    "autoinstall<enter>",
  ]

  cloud_init_user_data = <<-EOF
  #cloud-config
  autoinstall:
    version: 1
    identity:
      hostname: ubuntu
      username: ubuntu
      password: "$6$rounds=4096$abcdefghijklmnop$abcdefghijklmnopqrstuvabcdefghijklmnopqrstuvabcdefghijklmnopqrstuvabcdefghijklmnopqrstuvabcdefghijklmnopqrstuvabcdefghijklmnopqrstuv"
  EOF

  cloud_init_meta_data = <<-EOF
  instance-id: ubuntu-autoinstall
  local-hostname: ubuntu
  EOF
}

build {
  sources = ["source.harvester-iso.ubuntu"]

  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get install -y nginx",
    ]
  }
}
```
