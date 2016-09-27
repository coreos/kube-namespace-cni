// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

const configWithDefault = `
{
  "name": "kube-namespace",
  "type": "kube-namespace",
  "log_level": "debug",
  "namespaces": {
    "isolated": {
      "name": "isolated",
      "type": "bridge",
      "mtu": 1460,
      "addIf": "true",
      "isGateway": true,
      "ipMasq": true,
      "ipam": {
        "type": "host-local",
        "subnet": "10.2.0.0/16",
        "gateway": "10.2.0.1",
        "routes": [
          {
            "dst": "0.0.0.0/0"
          }
        ]
      }
    }
  },
  "default": {
    "name": "default-bridge",
    "type": "bridge",
    "bridge": "mybridge",
    "mtu": 1460,
    "addIf": "true",
    "isGateway": true,
    "ipMasq": true,
    "ipam": {
      "type": "host-local",
      "subnet": "10.1.0.0/16",
      "gateway": "10.1.0.1",
      "routes": [
        {
          "dst": "0.0.0.0/0"
        }
      ]
    }
  }
}
`

const configNoDefault = `
{
  "name": "kube-namespace",
  "type": "kube-namespace",
  "log_level": "debug",
  "namespaces": {
    "isolated": {
      "name": "isolated",
      "type": "bridge",
      "mtu": 1460,
      "addIf": "true",
      "isGateway": true,
      "ipMasq": true,
      "ipam": {
        "type": "host-local",
        "subnet": "10.2.0.0/16",
        "gateway": "10.2.0.1",
        "routes": [
          {
            "dst": "0.0.0.0/0"
          }
        ]
      }
    }
  }
}
`

// Parse CNI_ARGS correctly.
func TestParseExtraArgs(t *testing.T) {
	args := "K8S_POD_NAMESPACE=test;AnotherArg=123;BadArg"
	expected := map[string]string{
		"K8S_POD_NAMESPACE": "test",
		"AnotherArg":        "123",
	}

	assert.Equal(t, expected, parseExtraArgs(args))
}

// Return the correct namespace config.
func TestGetNamespaceConfig(t *testing.T) {
	config := &config{}
	if err := json.Unmarshal([]byte(configWithDefault), config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	netconf, err := config.getNetConf("K8S_POD_NAMESPACE=isolated")

	assert.NoError(t, err)
	assert.Equal(t, "bridge", netconf["type"].(string))
}

// Return the default config.
func TestGetDefaultConfig(t *testing.T) {
	config := &config{}
	if err := json.Unmarshal([]byte(configWithDefault), config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	netconf, err := config.getNetConf("K8S_POD_NAMESPACE=non-existent")

	assert.NoError(t, err)
	assert.Equal(t, "default-bridge", netconf["name"].(string))
}

// Error if no default.
func TestNoDefaultConfig(t *testing.T) {
	config := &config{}
	if err := json.Unmarshal([]byte(configNoDefault), config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	netconf, err := config.getNetConf("K8S_POD_NAMESPACE=non-existent")

	assert.Error(t, err)
	assert.Nil(t, netconf)
}

// Error if K8S_POD_NAMESPACE is empty.
func TestNoNamespace(t *testing.T) {
	config := &config{}
	_, err := config.getNetConf("")

	assert.Error(t, err)
}
