// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// generateSelfSignedCert returns base64-encoded PEM cert and key suitable for
// embedding in a kubeconfig.
func generateSelfSignedCert(t *testing.T) (certB64, keyB64 string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return base64.StdEncoding.EncodeToString(certPEM), base64.StdEncoding.EncodeToString(keyPEM)
}

func TestParseKubeconfig_Valid(t *testing.T) {
	kubeconfigYAML := `
apiVersion: v1
kind: Config
current-context: my-context
clusters:
- name: my-cluster
  cluster:
    server: https://192.168.1.100:6443
    insecure-skip-tls-verify: true
users:
- name: my-user
  user:
    token: "my-token-abc123"
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
    namespace: default
`
	kd, err := parseKubeconfig([]byte(kubeconfigYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kd.Server != "https://192.168.1.100:6443" {
		t.Errorf("expected server %q, got %q", "https://192.168.1.100:6443", kd.Server)
	}
	if kd.Token != "my-token-abc123" {
		t.Errorf("expected token %q, got %q", "my-token-abc123", kd.Token)
	}
	if !kd.SkipTLSVerify {
		t.Errorf("expected SkipTLSVerify to be true")
	}
	if kd.Namespace != "default" {
		t.Errorf("expected namespace %q, got %q", "default", kd.Namespace)
	}
}

func TestParseKubeconfig_MissingServer(t *testing.T) {
	kubeconfigYAML := `
apiVersion: v1
kind: Config
current-context: nonexistent-context
clusters: []
users: []
contexts: []
`
	_, err := parseKubeconfig([]byte(kubeconfigYAML))
	if err == nil {
		t.Fatal("expected error for missing server, got nil")
	}
}

func TestParseKubeconfig_EmptyToken(t *testing.T) {
	kubeconfigYAML := `
apiVersion: v1
kind: Config
current-context: my-context
clusters:
- name: my-cluster
  cluster:
    server: https://192.168.1.100:6443
users:
- name: my-user
  user: {}
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
`
	kd, err := parseKubeconfig([]byte(kubeconfigYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kd.Server != "https://192.168.1.100:6443" {
		t.Errorf("expected server %q, got %q", "https://192.168.1.100:6443", kd.Server)
	}
	if kd.Token != "" {
		t.Errorf("expected empty token, got %q", kd.Token)
	}
	if kd.SkipTLSVerify {
		t.Errorf("expected SkipTLSVerify to be false")
	}
}

func TestParseKubeconfig_ClientCertAuth(t *testing.T) {
	certB64, keyB64 := generateSelfSignedCert(t)

	kubeconfigYAML := `
apiVersion: v1
kind: Config
current-context: my-context
clusters:
- name: my-cluster
  cluster:
    server: https://192.168.1.100:6443
users:
- name: my-user
  user:
    client-certificate-data: ` + certB64 + `
    client-key-data: ` + keyB64 + `
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
`
	kd, err := parseKubeconfig([]byte(kubeconfigYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kd.Token != "" {
		t.Errorf("expected empty token for cert-based auth, got %q", kd.Token)
	}
	if len(kd.CertData) == 0 {
		t.Error("expected non-empty CertData")
	}
	if len(kd.KeyData) == 0 {
		t.Error("expected non-empty KeyData")
	}
}

func TestParseKubeconfig_CAData(t *testing.T) {
	certB64, _ := generateSelfSignedCert(t)

	kubeconfigYAML := `
apiVersion: v1
kind: Config
current-context: my-context
clusters:
- name: my-cluster
  cluster:
    server: https://192.168.1.100:6443
    certificate-authority-data: ` + certB64 + `
users:
- name: my-user
  user:
    token: "my-token"
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
`
	kd, err := parseKubeconfig([]byte(kubeconfigYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kd.CAData) == 0 {
		t.Error("expected non-empty CAData")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("https://192.168.1.100:6443", "default", "mytoken", true)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.BaseURL() != "https://192.168.1.100:6443" {
		t.Errorf("expected baseURL %q, got %q", "https://192.168.1.100:6443", c.BaseURL())
	}
	if c.VNCToken() != "mytoken" {
		t.Errorf("expected token %q, got %q", "mytoken", c.VNCToken())
	}
}

