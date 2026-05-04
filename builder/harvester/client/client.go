// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

// Package client provides a minimal HTTP client for interacting with the
// Harvester and Kubernetes APIs required by the packer builder.
package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// API group paths.
	kubevirtAPIPath    = "/apis/kubevirt.io/v1"
	subresourcesPath   = "/apis/subresources.kubevirt.io/v1"
	harvesterAPIPath   = "/apis/harvesterhci.io/v1beta1"
	cdiAPIPath         = "/apis/cdi.kubevirt.io/v1beta1"

	// StorageClass used by Harvester Longhorn.
	DefaultStorageClass = "harvester-longhorn"
)

// HarvesterClient is a minimal REST client for the Harvester/Kubernetes API.
type HarvesterClient struct {
	baseURL    string
	namespace  string
	token      string
	httpClient *http.Client
}

// NewClient creates a new HarvesterClient.
// harvesterURL should be the base URL of the Harvester API server,
// e.g. "https://192.168.1.100:6443".
func NewClient(harvesterURL, namespace, token string, skipTLSVerify bool) *HarvesterClient {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipTLSVerify, //nolint:gosec // controlled by user config
	}
	return newClientWithTLS(harvesterURL, namespace, token, tlsConfig)
}

// newClientWithTLS creates a HarvesterClient using the provided TLS configuration.
func newClientWithTLS(harvesterURL, namespace, token string, tlsCfg *tls.Config) *HarvesterClient {
	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}
	return &HarvesterClient{
		baseURL:   harvesterURL,
		namespace: namespace,
		token:     token,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
	}
}

// NewClientFromKubeconfig creates a HarvesterClient by reading a kubeconfig file.
// If kubeconfigPath is empty it falls back to $KUBECONFIG or ~/.kube/config.
func NewClientFromKubeconfig(kubeconfigPath, namespace string, skipTLSVerify bool) (*HarvesterClient, error) {
	return NewClientFromKubeconfigWithOverrides(kubeconfigPath, namespace, "", "", skipTLSVerify)
}

// NewClientFromKubeconfigWithOverrides creates a HarvesterClient from a kubeconfig file,
// optionally overriding the server URL and bearer token from the file.
//
//   - If overrideURL is non-empty it replaces the server URL from the kubeconfig.
//   - If overrideToken is non-empty it replaces the token (or client-certificate
//     credentials) from the kubeconfig. An explicit token always takes priority over
//     mTLS client-certificate credentials embedded in the kubeconfig.
//   - If the kubeconfig user has no bearer token and provides a client certificate +
//     key, mTLS authentication is used automatically.
//   - CA certificate data from the kubeconfig is always applied to the TLS config
//     (unless skipTLSVerify is true).
//
// If kubeconfigPath is empty the function falls back to $KUBECONFIG or ~/.kube/config.
func NewClientFromKubeconfigWithOverrides(kubeconfigPath, namespace, overrideURL, overrideToken string, skipTLSVerify bool) (*HarvesterClient, error) {
	if kubeconfigPath == "" {
		if env := os.Getenv("KUBECONFIG"); env != "" {
			kubeconfigPath = env
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("cannot determine home directory: %w", err)
			}
			kubeconfigPath = home + "/.kube/config"
		}
	}

	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read kubeconfig %s: %w", kubeconfigPath, err)
	}

	kd, err := parseKubeconfig(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse kubeconfig: %w", err)
	}

	// Apply explicit overrides.
	server := kd.Server
	if overrideURL != "" {
		server = overrideURL
	}

	token := kd.Token
	if overrideToken != "" {
		token = overrideToken
	}

	// Resolve namespace: explicit arg > kubeconfig context > "default".
	ns := namespace
	if ns == "" {
		ns = kd.Namespace
	}
	if ns == "" {
		ns = "default"
	}

	// Build TLS config.
	tlsCfg := &tls.Config{
		InsecureSkipVerify: skipTLSVerify || kd.SkipTLSVerify, //nolint:gosec // controlled by user config
	}

	// Add CA bundle from kubeconfig (ignored when InsecureSkipVerify is set).
	if len(kd.CAData) > 0 && !tlsCfg.InsecureSkipVerify {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(kd.CAData) {
			return nil, fmt.Errorf("kubeconfig certificate-authority-data contains no valid PEM certificates")
		}
		tlsCfg.RootCAs = pool
	}

	// Use mTLS when no bearer token is available and the kubeconfig provides a
	// client certificate + key pair. An explicit overrideToken takes priority.
	// Note: passing an empty string as overrideToken is treated the same as
	// "no override", so mTLS credentials from the kubeconfig are used when the
	// kubeconfig itself also has no bearer token.
	if token == "" && len(kd.CertData) > 0 && len(kd.KeyData) > 0 {
		cert, certErr := tls.X509KeyPair(kd.CertData, kd.KeyData)
		if certErr != nil {
			return nil, fmt.Errorf("parse client certificate from kubeconfig: %w", certErr)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return newClientWithTLS(server, ns, token, tlsCfg), nil
}

// request performs an authenticated HTTP request and returns the raw body bytes.
func (c *HarvesterClient) request(method, path string, body interface{}) ([]byte, int, error) {
	return c.requestWithAccept(method, path, body, "application/json")
}

// requestWithAccept performs an authenticated HTTP request with a custom
// Accept header and returns the raw body bytes.
func (c *HarvesterClient) requestWithAccept(method, path string, body interface{}, accept string) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var statusErr StatusError
		if jsonErr := json.Unmarshal(respBody, &statusErr); jsonErr == nil && statusErr.Message != "" {
			return nil, resp.StatusCode, &statusErr
		}
		return nil, resp.StatusCode, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

// patch sends a JSON merge-patch PATCH request.
func (c *HarvesterClient) patch(path string, body interface{}) ([]byte, int, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal patch body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("create patch request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute patch %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read patch response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var statusErr StatusError
		if jsonErr := json.Unmarshal(respBody, &statusErr); jsonErr == nil && statusErr.Message != "" {
			return nil, resp.StatusCode, &statusErr
		}
		return nil, resp.StatusCode, fmt.Errorf("API patch error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

// --- VirtualMachine operations ---

// vmPath returns the API path for VMs.
func (c *HarvesterClient) vmPath(name string) string {
	if name == "" {
		return fmt.Sprintf("%s/namespaces/%s/virtualmachines", kubevirtAPIPath, c.namespace)
	}
	return fmt.Sprintf("%s/namespaces/%s/virtualmachines/%s", kubevirtAPIPath, c.namespace, name)
}

// CreateVM creates a VirtualMachine resource.
func (c *HarvesterClient) CreateVM(vm *VirtualMachine) (*VirtualMachine, error) {
	body, _, err := c.request(http.MethodPost, c.vmPath(""), vm)
	if err != nil {
		return nil, fmt.Errorf("create VM: %w", err)
	}
	var result VirtualMachine
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal VM: %w", err)
	}
	return &result, nil
}

// GetVM retrieves a VirtualMachine by name.
func (c *HarvesterClient) GetVM(name string) (*VirtualMachine, error) {
	body, _, err := c.request(http.MethodGet, c.vmPath(name), nil)
	if err != nil {
		return nil, fmt.Errorf("get VM %s: %w", name, err)
	}
	var vm VirtualMachine
	if err := json.Unmarshal(body, &vm); err != nil {
		return nil, fmt.Errorf("unmarshal VM: %w", err)
	}
	return &vm, nil
}

// StartVM invokes the VM start subresource.
func (c *HarvesterClient) StartVM(name string) error {
	path := fmt.Sprintf("%s/namespaces/%s/virtualmachines/%s/start",
		subresourcesPath, c.namespace, name)
	_, _, err := c.requestWithAccept(http.MethodPut, path, struct{}{}, "*/*")
	if err != nil {
		return fmt.Errorf("start VM %s: %w", name, err)
	}
	return nil
}

// StopVM stops the VM gracefully.
func (c *HarvesterClient) StopVM(name string) error {
	path := fmt.Sprintf("%s/namespaces/%s/virtualmachines/%s/stop",
		subresourcesPath, c.namespace, name)
	_, _, err := c.requestWithAccept(http.MethodPut, path, struct{}{}, "*/*")
	if err != nil {
		return fmt.Errorf("stop VM %s: %w", name, err)
	}
	return nil
}

// DeleteVM deletes a VirtualMachine.
func (c *HarvesterClient) DeleteVM(name string) error {
	_, _, err := c.request(http.MethodDelete, c.vmPath(name), nil)
	if err != nil {
		return fmt.Errorf("delete VM %s: %w", name, err)
	}
	return nil
}

// --- VirtualMachineInstance operations ---

// vmiPath returns the API path for VMIs.
func (c *HarvesterClient) vmiPath(name string) string {
	if name == "" {
		return fmt.Sprintf("%s/namespaces/%s/virtualmachineinstances", kubevirtAPIPath, c.namespace)
	}
	return fmt.Sprintf("%s/namespaces/%s/virtualmachineinstances/%s", kubevirtAPIPath, c.namespace, name)
}

// GetVMI retrieves a VirtualMachineInstance by name.
func (c *HarvesterClient) GetVMI(name string) (*VirtualMachineInstance, error) {
	body, _, err := c.request(http.MethodGet, c.vmiPath(name), nil)
	if err != nil {
		return nil, fmt.Errorf("get VMI %s: %w", name, err)
	}
	var vmi VirtualMachineInstance
	if err := json.Unmarshal(body, &vmi); err != nil {
		return nil, fmt.Errorf("unmarshal VMI: %w", err)
	}
	return &vmi, nil
}

// VNCWebSocketURL returns the URL for the VNC WebSocket subresource.
func (c *HarvesterClient) VNCWebSocketURL(vmiName string) string {
	return fmt.Sprintf("%s/namespaces/%s/virtualmachineinstances/%s/vnc",
		subresourcesPath, c.namespace, vmiName)
}

// VNCToken returns the bearer token used for authentication.
func (c *HarvesterClient) VNCToken() string {
	return c.token
}

// BaseURL returns the base URL of the Harvester server.
func (c *HarvesterClient) BaseURL() string {
	return c.baseURL
}

// HTTPClient returns the underlying HTTP client (needed for WebSocket dial).
func (c *HarvesterClient) HTTPClient() *http.Client {
	return c.httpClient
}

// --- DataVolume operations ---

// dvPath returns the API path for DataVolumes.
func (c *HarvesterClient) dvPath(name string) string {
	if name == "" {
		return fmt.Sprintf("%s/namespaces/%s/datavolumes", cdiAPIPath, c.namespace)
	}
	return fmt.Sprintf("%s/namespaces/%s/datavolumes/%s", cdiAPIPath, c.namespace, name)
}

// CreateDataVolume creates a DataVolume resource.
func (c *HarvesterClient) CreateDataVolume(dv *DataVolume) (*DataVolume, error) {
	body, _, err := c.request(http.MethodPost, c.dvPath(""), dv)
	if err != nil {
		return nil, fmt.Errorf("create DataVolume: %w", err)
	}
	var result DataVolume
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal DataVolume: %w", err)
	}
	return &result, nil
}

// GetDataVolume retrieves a DataVolume by name.
func (c *HarvesterClient) GetDataVolume(name string) (*DataVolume, error) {
	body, _, err := c.request(http.MethodGet, c.dvPath(name), nil)
	if err != nil {
		return nil, fmt.Errorf("get DataVolume %s: %w", name, err)
	}
	var dv DataVolume
	if err := json.Unmarshal(body, &dv); err != nil {
		return nil, fmt.Errorf("unmarshal DataVolume: %w", err)
	}
	return &dv, nil
}

// DeleteDataVolume deletes a DataVolume.
func (c *HarvesterClient) DeleteDataVolume(name string) error {
	_, _, err := c.request(http.MethodDelete, c.dvPath(name), nil)
	if err != nil {
		return fmt.Errorf("delete DataVolume %s: %w", name, err)
	}
	return nil
}

// --- VirtualMachineImage operations ---

// imagePath returns the API path for VirtualMachineImages.
func (c *HarvesterClient) imagePath(namespace, name string) string {
	if namespace == "" {
		namespace = c.namespace
	}
	if name == "" {
		return fmt.Sprintf("%s/namespaces/%s/virtualmachineimages", harvesterAPIPath, namespace)
	}
	return fmt.Sprintf("%s/namespaces/%s/virtualmachineimages/%s", harvesterAPIPath, namespace, name)
}

// CreateVMImage creates a VirtualMachineImage resource.
func (c *HarvesterClient) CreateVMImage(img *VirtualMachineImage) (*VirtualMachineImage, error) {
	body, _, err := c.request(http.MethodPost, c.imagePath(img.ObjectMeta.Namespace, ""), img)
	if err != nil {
		return nil, fmt.Errorf("create VirtualMachineImage: %w", err)
	}
	var result VirtualMachineImage
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal VirtualMachineImage: %w", err)
	}
	return &result, nil
}

// GetVMImage retrieves a VirtualMachineImage by namespace/name.
func (c *HarvesterClient) GetVMImage(namespace, name string) (*VirtualMachineImage, error) {
	body, _, err := c.request(http.MethodGet, c.imagePath(namespace, name), nil)
	if err != nil {
		return nil, fmt.Errorf("get VirtualMachineImage %s/%s: %w", namespace, name, err)
	}
	var img VirtualMachineImage
	if err := json.Unmarshal(body, &img); err != nil {
		return nil, fmt.Errorf("unmarshal VirtualMachineImage: %w", err)
	}
	return &img, nil
}

// ListVMImages lists VirtualMachineImages in a namespace.
func (c *HarvesterClient) ListVMImages(namespace string) (*VirtualMachineImageList, error) {
	body, _, err := c.request(http.MethodGet, c.imagePath(namespace, ""), nil)
	if err != nil {
		return nil, fmt.Errorf("list VirtualMachineImages: %w", err)
	}
	var list VirtualMachineImageList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("unmarshal VirtualMachineImageList: %w", err)
	}
	return &list, nil
}

// DeleteVMImage deletes a VirtualMachineImage.
func (c *HarvesterClient) DeleteVMImage(namespace, name string) error {
	_, _, err := c.request(http.MethodDelete, c.imagePath(namespace, name), nil)
	if err != nil {
		return fmt.Errorf("delete VirtualMachineImage %s/%s: %w", namespace, name, err)
	}
	return nil
}

// GetVMImageByDisplayName finds a VirtualMachineImage by its display name.
func (c *HarvesterClient) GetVMImageByDisplayName(namespace, displayName string) (*VirtualMachineImage, error) {
	list, err := c.ListVMImages(namespace)
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].Spec.DisplayName == displayName || list.Items[i].ObjectMeta.Name == displayName {
			return &list.Items[i], nil
		}
	}
	return nil, fmt.Errorf("image %q not found in namespace %s", displayName, namespace)
}
