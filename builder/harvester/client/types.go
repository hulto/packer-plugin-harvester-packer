// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package client

// ObjectMeta represents Kubernetes object metadata.
type ObjectMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	UID         string            `json:"uid,omitempty"`
}

// TypeMeta holds the API version and kind.
type TypeMeta struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

// ResourceRequirements defines resource requests/limits.
type ResourceRequirements struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// CPU configuration.
type CPU struct {
	Cores   int `json:"cores,omitempty"`
	Sockets int `json:"sockets,omitempty"`
	Threads int `json:"threads,omitempty"`
}

// Disk device spec.
type Disk struct {
	Bus string `json:"bus,omitempty"`
}

// CDRom device spec.
type CDRom struct {
	Bus      string `json:"bus,omitempty"`
	ReadOnly bool   `json:"readonly,omitempty"`
}

// DiskTarget describes a disk device.
type DiskTarget struct {
	Name      string `json:"name"`
	BootOrder uint   `json:"bootOrder,omitempty"`
	Disk      *Disk  `json:"disk,omitempty"`
	CDRom     *CDRom `json:"cdrom,omitempty"`
}

// Interface device spec.
type Interface struct {
	Name       string      `json:"name"`
	Model      string      `json:"model,omitempty"`
	Masquerade interface{} `json:"masquerade,omitempty"`
	Bridge     interface{} `json:"bridge,omitempty"`
}

// Devices spec.
type Devices struct {
	Disks      []DiskTarget `json:"disks,omitempty"`
	Interfaces []Interface  `json:"interfaces,omitempty"`
}

// Domain spec.
type Domain struct {
	CPU       CPU                  `json:"cpu,omitempty"`
	Resources ResourceRequirements `json:"resources,omitempty"`
	Devices   Devices              `json:"devices,omitempty"`
}

// Network spec.
type Network struct {
	Name        string       `json:"name"`
	Pod         *PodNetwork  `json:"pod,omitempty"`
	Multus      *MultusNet   `json:"multus,omitempty"`
}

// PodNetwork references the default pod network.
type PodNetwork struct{}

// MultusNet references a Multus CNI network.
type MultusNet struct {
	NetworkName string `json:"networkName"`
	Default     bool   `json:"default,omitempty"`
}

// DataVolumeSource references a DataVolume.
type DataVolumeSource struct {
	Name string `json:"name"`
}

// PersistentVolumeClaimVolumeSource references a PVC.
type PersistentVolumeClaimVolumeSource struct {
	ClaimName string `json:"claimName"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// CloudInitNoCloud inlines cloud-init config.
type CloudInitNoCloud struct {
	UserData string `json:"userData,omitempty"`
}

// ContainerDiskSource references a container image.
type ContainerDiskSource struct {
	Image string `json:"image"`
}

// Volume spec.
type Volume struct {
	Name                  string                             `json:"name"`
	DataVolume            *DataVolumeSource                  `json:"dataVolume,omitempty"`
	PersistentVolumeClaim *PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
	CloudInitNoCloud      *CloudInitNoCloud                  `json:"cloudInitNoCloud,omitempty"`
	ContainerDisk         *ContainerDiskSource               `json:"containerDisk,omitempty"`
}

// VMSpec defines the desired VM spec.
type VMSpec struct {
	Domain   Domain    `json:"domain"`
	Networks []Network `json:"networks,omitempty"`
	Volumes  []Volume  `json:"volumes,omitempty"`
}

// VMTemplateSpec is the template for the VMI.
type VMTemplateSpec struct {
	ObjectMeta ObjectMeta `json:"metadata,omitempty"`
	Spec       VMSpec     `json:"spec,omitempty"`
}

// VirtualMachineSpec defines VM desired state.
type VirtualMachineSpec struct {
	RunStrategy string         `json:"runStrategy,omitempty"`
	Template    VMTemplateSpec `json:"template,omitempty"`
}

// VirtualMachineStatus describes VM status.
type VirtualMachineStatus struct {
	Ready bool   `json:"ready,omitempty"`
	Phase string `json:"printableStatus,omitempty"`
}

// VirtualMachine represents a Kubernetes VirtualMachine resource.
type VirtualMachine struct {
	TypeMeta   `json:",inline"`
	ObjectMeta ObjectMeta           `json:"metadata,omitempty"`
	Spec       VirtualMachineSpec   `json:"spec,omitempty"`
	Status     VirtualMachineStatus `json:"status,omitempty"`
}

// VMIPhase describes the phase of a VMI.
type VMIPhase string

const (
	VMIPhasePending    VMIPhase = "Pending"
	VMIPhaseScheduling VMIPhase = "Scheduling"
	VMIPhaseScheduled  VMIPhase = "Scheduled"
	VMIPhaseRunning    VMIPhase = "Running"
	VMIPhaseSucceeded  VMIPhase = "Succeeded"
	VMIPhaseFailed     VMIPhase = "Failed"
	VMIPhaseUnknown    VMIPhase = "Unknown"
)

// VMIStatus describes VMI status.
type VMIStatus struct {
	Phase      VMIPhase `json:"phase,omitempty"`
	NodeName   string   `json:"nodeName,omitempty"`
	Interfaces []VMIIface `json:"interfaces,omitempty"`
}

// VMIIface describes a VMI network interface.
type VMIIface struct {
	Name      string `json:"name,omitempty"`
	IPAddress string `json:"ipAddress,omitempty"`
}

// VirtualMachineInstance represents a running VMI.
type VirtualMachineInstance struct {
	TypeMeta   `json:",inline"`
	ObjectMeta ObjectMeta `json:"metadata,omitempty"`
	Status     VMIStatus  `json:"status,omitempty"`
}

// PVCSpec describes a PersistentVolumeClaim spec.
type PVCSpec struct {
	AccessModes      []string             `json:"accessModes,omitempty"`
	Resources        ResourceRequirements `json:"resources,omitempty"`
	VolumeMode       string               `json:"volumeMode,omitempty"`
	StorageClassName string               `json:"storageClassName,omitempty"`
}

// PersistentVolumeClaim is a K8s PVC.
type PersistentVolumeClaim struct {
	TypeMeta   `json:",inline"`
	ObjectMeta ObjectMeta `json:"metadata,omitempty"`
	Spec       PVCSpec    `json:"spec,omitempty"`
}

// DataVolumeSpec is the spec for a DataVolume.
type DataVolumeSpec struct {
	Source *DataVolumeSpecSource `json:"source,omitempty"`
	PVC    PVCSpec               `json:"pvc,omitempty"`
}

// DataVolumeSpecSource describes the data source for a DataVolume.
type DataVolumeSpecSource struct {
	HTTP     *DataVolumeSourceHTTP  `json:"http,omitempty"`
	PVC      *DataVolumeSourcePVC   `json:"pvc,omitempty"`
	Blank    *DataVolumeSourceBlank `json:"blank,omitempty"`
	Registry *DataVolumeSourceReg   `json:"registry,omitempty"`
}

// DataVolumeSourceHTTP imports data from a URL.
type DataVolumeSourceHTTP struct {
	URL string `json:"url"`
}

// DataVolumeSourcePVC clones from an existing PVC.
type DataVolumeSourcePVC struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// DataVolumeSourceBlank creates a blank disk.
type DataVolumeSourceBlank struct{}

// DataVolumeSourceReg imports from a container registry.
type DataVolumeSourceReg struct {
	URL string `json:"url"`
}

// DataVolumeStatus describes DataVolume phase.
type DataVolumeStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

// DataVolume is a CDI DataVolume resource.
type DataVolume struct {
	TypeMeta   `json:",inline"`
	ObjectMeta ObjectMeta       `json:"metadata,omitempty"`
	Spec       DataVolumeSpec   `json:"spec,omitempty"`
	Status     DataVolumeStatus `json:"status,omitempty"`
}

// VirtualMachineImageSource describes the source of an image.
type VirtualMachineImageSource struct {
	Type        string `json:"type"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

// VirtualMachineImageSpec is the spec for a Harvester VirtualMachineImage.
type VirtualMachineImageSpec struct {
	DisplayName  string                    `json:"displayName,omitempty"`
	SourceType   string                    `json:"sourceType,omitempty"`
	URL          string                    `json:"url,omitempty"`
	PVCName      string                    `json:"pvcName,omitempty"`
	PVCNamespace string                    `json:"pvcNamespace,omitempty"`
}

// VirtualMachineImageStatus describes image status.
type VirtualMachineImageStatus struct {
	Phase            string `json:"phase,omitempty"`
	Message          string `json:"message,omitempty"`
	Size             int64  `json:"size,omitempty"`
	StorageClassName string `json:"storageClassName,omitempty"`
}

// VirtualMachineImage is a Harvester image CRD.
type VirtualMachineImage struct {
	TypeMeta   `json:",inline"`
	ObjectMeta ObjectMeta                `json:"metadata,omitempty"`
	Spec       VirtualMachineImageSpec   `json:"spec,omitempty"`
	Status     VirtualMachineImageStatus `json:"status,omitempty"`
}

// VirtualMachineImageList is a list of VirtualMachineImage.
type VirtualMachineImageList struct {
	TypeMeta `json:",inline"`
	Items    []VirtualMachineImage `json:"items"`
}

// StatusError represents a Kubernetes API error response.
type StatusError struct {
	TypeMeta `json:",inline"`
	Message  string `json:"message,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Code     int    `json:"code,omitempty"`
}

func (e *StatusError) Error() string {
	return e.Message
}
