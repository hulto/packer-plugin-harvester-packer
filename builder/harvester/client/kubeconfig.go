// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/base64"
	"fmt"

	"gopkg.in/yaml.v2"
)

// kubeconfigData holds the values extracted from a kubeconfig file.
type kubeconfigData struct {
	// Server is the API server URL.
	Server string
	// Token is the bearer token for authentication (may be empty when mTLS is used).
	Token string
	// CAData is the PEM-encoded CA certificate bundle used to verify the server (may be nil).
	CAData []byte
	// CertData is the PEM-encoded client certificate for mTLS authentication (may be nil).
	CertData []byte
	// KeyData is the PEM-encoded client private key for mTLS authentication (may be nil).
	KeyData []byte
	// SkipTLSVerify disables server certificate verification.
	SkipTLSVerify bool
	// Namespace is the default namespace from the active context (may be empty).
	Namespace string
}

// minimalKubeconfig is a minimal representation for parsing a kubeconfig file.
type minimalKubeconfig struct {
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Token          string `yaml:"token"`
			ClientCertData string `yaml:"client-certificate-data"`
			ClientKeyData  string `yaml:"client-key-data"`
		} `yaml:"user"`
	} `yaml:"users"`
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster   string `yaml:"cluster"`
			User      string `yaml:"user"`
			Namespace string `yaml:"namespace"`
		} `yaml:"context"`
	} `yaml:"contexts"`
}

// parseKubeconfig extracts connection settings from raw kubeconfig YAML bytes.
// It returns a kubeconfigData struct containing the server URL, authentication
// credentials (bearer token or mTLS client cert/key), CA data, and TLS settings.
func parseKubeconfig(data []byte) (*kubeconfigData, error) {
	var kc minimalKubeconfig
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return nil, fmt.Errorf("unmarshal kubeconfig: %w", err)
	}

	// Determine active context.
	contextName := kc.CurrentContext
	var clusterName, userName, namespace string
	for _, ctx := range kc.Contexts {
		if ctx.Name == contextName {
			clusterName = ctx.Context.Cluster
			userName = ctx.Context.User
			namespace = ctx.Context.Namespace
			break
		}
	}

	result := &kubeconfigData{
		Namespace: namespace,
	}

	// Resolve cluster settings.
	for _, cl := range kc.Clusters {
		if cl.Name == clusterName {
			result.Server = cl.Cluster.Server
			result.SkipTLSVerify = cl.Cluster.InsecureSkipTLSVerify
			if cl.Cluster.CertificateAuthorityData != "" {
				caBytes, err := base64.StdEncoding.DecodeString(cl.Cluster.CertificateAuthorityData)
				if err != nil {
					return nil, fmt.Errorf("decode certificate-authority-data: %w", err)
				}
				result.CAData = caBytes
			}
			break
		}
	}

	if result.Server == "" {
		return nil, fmt.Errorf("could not find server for context %q in kubeconfig", contextName)
	}

	// Resolve user credentials.
	for _, u := range kc.Users {
		if u.Name == userName {
			result.Token = u.User.Token

			if u.User.ClientCertData != "" {
				certBytes, err := base64.StdEncoding.DecodeString(u.User.ClientCertData)
				if err != nil {
					return nil, fmt.Errorf("decode client-certificate-data: %w", err)
				}
				result.CertData = certBytes
			}

			if u.User.ClientKeyData != "" {
				keyBytes, err := base64.StdEncoding.DecodeString(u.User.ClientKeyData)
				if err != nil {
					return nil, fmt.Errorf("decode client-key-data: %w", err)
				}
				result.KeyData = keyBytes
			}
			break
		}
	}

	return result, nil
}
