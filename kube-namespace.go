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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"

	"github.com/Sirupsen/logrus"
)

var log = logrus.NewEntry(logrus.New())

type config struct {
	Name       string
	Type       string
	LogLevel   string `json:"log_level"`
	Default    map[string]interface{}
	Namespaces map[string]map[string]interface{}
}

// Return the network config for the given namespace, or the default
// config if no per-namespace config is found.  If the no config is
// found for the namespace and no default is specified, return an
// error.
func (c *config) getNetConf(args string) (map[string]interface{}, error) {
	extraArgs := parseExtraArgs(args)
	namespace, pod := extraArgs["K8S_POD_NAMESPACE"], extraArgs["K8S_POD_NAME"]

	if namespace == "" {
		return nil, errors.New("Kubernetes namespace argument missing or empty.")
	}

	if cfg, ok := c.Namespaces[namespace]; ok {
		log.WithFields(logrus.Fields{
			"namespace": namespace,
			"pod":       pod,
			"config":    cfg,
		}).Debug("Using namespace specific config.")

		return cfg, nil
	}

	if len(c.Default) == 0 {
		return nil,
			fmt.Errorf("Config for namespace %q not found, and no default given.", namespace)
	}

	log.WithFields(logrus.Fields{
		"namespace": namespace,
		"pod":       pod,
		"config":    c.Default,
	}).Debug("Per-namespace config not found. Using default.")

	return c.Default, nil
}

func (c *config) setLogLevel() {
	if c.LogLevel == "" {
		return
	}

	if logLevel, err := logrus.ParseLevel(c.LogLevel); err != nil {
		log.Error("Unknown log level. Using default: INFO")
	} else {
		log.Logger.Level = logLevel
	}
}

// Parse extra arguments passed in the CNI_ARGS environment variable.
// Kubernetes uses this to provide the pod name and namespace.
func parseExtraArgs(args string) map[string]string {
	parsedArgs := make(map[string]string)

	for _, s := range strings.Split(args, ";") {
		s := strings.SplitN(s, "=", 2)
		if len(s) < 2 {
			continue
		}

		k, v := s[0], s[1]
		parsedArgs[k] = v
	}

	return parsedArgs
}

func delegateAdd(netconf map[string]interface{}) error {
	ncBytes, err := json.Marshal(netconf)
	if err != nil {
		return fmt.Errorf("Failed to marshal config: %v", err)
	}

	result, err := invoke.DelegateAdd(netconf["type"].(string), ncBytes)
	if err != nil {
		return err
	}

	return result.Print()
}

func delegateDel(netconf map[string]interface{}) error {
	ncBytes, err := json.Marshal(netconf)
	if err != nil {
		return fmt.Errorf("Failed to marshal config: %v", err)
	}

	return invoke.DelegateDel(netconf["type"].(string), ncBytes)
}

func cmdAdd(args *skel.CmdArgs) error {
	config := &config{}
	if err := json.Unmarshal(args.StdinData, config); err != nil {
		return fmt.Errorf("Failed to parse config: %v", err)
	}

	config.setLogLevel()
	log = log.WithFields(logrus.Fields{"container_id": args.ContainerID})
	log.Info("Configuring pod networking.")

	delegatedConfig, err := config.getNetConf(args.Args)
	if err != nil {
		return err
	}

	return delegateAdd(delegatedConfig)
}

func cmdDel(args *skel.CmdArgs) error {
	config := &config{}
	if err := json.Unmarshal(args.StdinData, config); err != nil {
		return fmt.Errorf("Failed to parse config: %v", err)
	}

	config.setLogLevel()
	log = log.WithFields(logrus.Fields{"container_id": args.ContainerID})
	log.Info("Removing pod networking.")

	delegatedConfig, err := config.getNetConf(args.Args)
	if err != nil {
		return err
	}

	return delegateDel(delegatedConfig)
}

func main() {
	logrus.SetOutput(os.Stderr)
	skel.PluginMain(cmdAdd, cmdDel, version.Legacy)
}
