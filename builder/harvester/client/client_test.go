// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"testing"
)

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
	server, token, skipTLS, err := parseKubeconfig([]byte(kubeconfigYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "https://192.168.1.100:6443" {
		t.Errorf("expected server %q, got %q", "https://192.168.1.100:6443", server)
	}
	if token != "my-token-abc123" {
		t.Errorf("expected token %q, got %q", "my-token-abc123", token)
	}
	if !skipTLS {
		t.Errorf("expected skipTLS to be true")
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
	_, _, _, err := parseKubeconfig([]byte(kubeconfigYAML))
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
	server, token, skipTLS, err := parseKubeconfig([]byte(kubeconfigYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "https://192.168.1.100:6443" {
		t.Errorf("expected server %q, got %q", "https://192.168.1.100:6443", server)
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if skipTLS {
		t.Errorf("expected skipTLS to be false")
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
