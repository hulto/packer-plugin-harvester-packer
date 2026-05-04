// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/base64"
	"fmt"

	"gopkg.in/yaml.v2"
)

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
			Token               string `yaml:"token"`
			ClientCertData      string `yaml:"client-certificate-data"`
			ClientKeyData       string `yaml:"client-key-data"`
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

// parseKubeconfig extracts server URL, token/cert auth, and TLS settings
// from raw kubeconfig YAML bytes.
func parseKubeconfig(data []byte) (server, token string, skipTLS bool, err error) {
	var kc minimalKubeconfig
	if err = yaml.Unmarshal(data, &kc); err != nil {
		return "", "", false, fmt.Errorf("unmarshal kubeconfig: %w", err)
	}

	// Determine active context.
	contextName := kc.CurrentContext
	var clusterName, userName string
	for _, ctx := range kc.Contexts {
		if ctx.Name == contextName {
			clusterName = ctx.Context.Cluster
			userName = ctx.Context.User
			break
		}
	}

	// Resolve cluster server.
	for _, cl := range kc.Clusters {
		if cl.Name == clusterName {
			server = cl.Cluster.Server
			skipTLS = cl.Cluster.InsecureSkipTLSVerify
			break
		}
	}

	if server == "" {
		return "", "", false, fmt.Errorf("could not find server for context %q in kubeconfig", contextName)
	}

	// Resolve user token.
	for _, u := range kc.Users {
		if u.Name == userName {
			token = u.User.Token
			// If no token, try to construct from cert/key (not implemented here –
			// users should supply a service-account token for the plugin).
			if token == "" && u.User.ClientCertData != "" {
				// Validate the base64 is parseable; actual mTLS dialing is outside scope.
				_, decErr := base64.StdEncoding.DecodeString(u.User.ClientCertData)
				if decErr != nil {
					return "", "", false, fmt.Errorf("client cert in kubeconfig is not valid base64: %w", decErr)
				}
				// Leave token empty; callers that need mTLS should use a token instead.
			}
			break
		}
	}

	return server, token, skipTLS, nil
}
