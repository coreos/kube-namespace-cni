// Copyright 2015 CNI authors
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
	"github.com/containernetworking/cni/plugins/ipam/host-local/backend/disk"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
)

func main() {
	skel.PluginMain(cmdAdd, cmdDel, version.Legacy)
}

func cmdAdd(args *skel.CmdArgs) error {
	ipamConf, err := LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	store, err := disk.New(ipamConf.Name)
	if err != nil {
		return err
	}
	defer store.Close()

	allocator, err := NewIPAllocator(ipamConf, store)
	if err != nil {
		return err
	}

	ipConf, err := allocator.Get(args.ContainerID)
	if err != nil {
		return err
	}

	r := &types.Result{
		IP4: ipConf,
	}
	return r.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	ipamConf, err := LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	store, err := disk.New(ipamConf.Name)
	if err != nil {
		return err
	}
	defer store.Close()

	allocator, err := NewIPAllocator(ipamConf, store)
	if err != nil {
		return err
	}

	return allocator.Release(args.ContainerID)
}
