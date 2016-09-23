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
	"fmt"
	"net"
	"syscall"

	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/testutils"
	"github.com/containernetworking/cni/pkg/types"

	"github.com/containernetworking/cni/pkg/utils/hwaddr"

	"github.com/vishvananda/netlink"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("bridge Operations", func() {
	var originalNS ns.NetNS

	BeforeEach(func() {
		// Create a new NetNS so we don't modify the host
		var err error
		originalNS, err = ns.NewNS()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(originalNS.Close()).To(Succeed())
	})

	It("creates a bridge", func() {
		const IFNAME = "bridge0"

		conf := &NetConf{
			NetConf: types.NetConf{
				CNIVersion: "0.2.0",
				Name:       "testConfig",
				Type:       "bridge",
			},
			BrName: IFNAME,
			IsGW:   false,
			IPMasq: false,
			MTU:    5000,
		}

		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			bridge, err := setupBridge(conf)
			Expect(err).NotTo(HaveOccurred())
			Expect(bridge.Attrs().Name).To(Equal(IFNAME))

			// Double check that the link was added
			link, err := netlink.LinkByName(IFNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(IFNAME))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("handles an existing bridge", func() {
		const IFNAME = "bridge0"

		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err := netlink.LinkAdd(&netlink.Bridge{
				LinkAttrs: netlink.LinkAttrs{
					Name: IFNAME,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			link, err := netlink.LinkByName(IFNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(IFNAME))
			ifindex := link.Attrs().Index

			conf := &NetConf{
				NetConf: types.NetConf{
					CNIVersion: "0.2.0",
					Name:       "testConfig",
					Type:       "bridge",
				},
				BrName: IFNAME,
				IsGW:   false,
				IPMasq: false,
			}

			bridge, err := setupBridge(conf)
			Expect(err).NotTo(HaveOccurred())
			Expect(bridge.Attrs().Name).To(Equal(IFNAME))
			Expect(bridge.Attrs().Index).To(Equal(ifindex))

			// Double check that the link has the same ifindex
			link, err = netlink.LinkByName(IFNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(IFNAME))
			Expect(link.Attrs().Index).To(Equal(ifindex))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("configures and deconfigures a bridge and veth with default route with ADD/DEL", func() {
		const BRNAME = "cni0"
		const IFNAME = "eth0"

		gwaddr, subnet, err := net.ParseCIDR("10.1.2.1/24")
		Expect(err).NotTo(HaveOccurred())

		conf := fmt.Sprintf(`{
    "cniVersion": "0.2.0",
    "name": "mynet",
    "type": "bridge",
    "bridge": "%s",
    "isDefaultGateway": true,
    "ipMasq": false,
    "ipam": {
        "type": "host-local",
        "subnet": "%s"
    }
}`, BRNAME, subnet.String())

		targetNs, err := ns.NewNS()
		Expect(err).NotTo(HaveOccurred())
		defer targetNs.Close()

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       targetNs.Path(),
			IfName:      IFNAME,
			StdinData:   []byte(conf),
		}

		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			_, err := testutils.CmdAddWithResult(targetNs.Path(), IFNAME, func() error {
				return cmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())

			// Make sure bridge link exists
			link, err := netlink.LinkByName(BRNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(BRNAME))

			// Ensure bridge has gateway address
			addrs, err := netlink.AddrList(link, syscall.AF_INET)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(addrs)).To(BeNumerically(">", 0))
			found := false
			subnetPrefix, subnetBits := subnet.Mask.Size()
			for _, a := range addrs {
				aPrefix, aBits := a.IPNet.Mask.Size()
				if a.IPNet.IP.Equal(gwaddr) && aPrefix == subnetPrefix && aBits == subnetBits {
					found = true
					break
				}
			}
			Expect(found).To(Equal(true))

			// Check for the veth link in the main namespace
			links, err := netlink.LinkList()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(links)).To(Equal(3)) // Bridge, veth, and loopback
			for _, l := range links {
				switch {
				case l.Attrs().Name == BRNAME:
					{
						_, isBridge := l.(*netlink.Bridge)
						Expect(isBridge).To(Equal(true))
						hwAddr := fmt.Sprintf("%s", l.Attrs().HardwareAddr)
						Expect(hwAddr).To(HavePrefix(hwaddr.PrivateMACPrefixString))
					}
				case l.Attrs().Name != BRNAME && l.Attrs().Name != "lo":
					{
						_, isVeth := l.(*netlink.Veth)
						Expect(isVeth).To(Equal(true))
					}
				}
			}
			Expect(err).NotTo(HaveOccurred())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Find the veth peer in the container namespace and the default route
		err = targetNs.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			link, err := netlink.LinkByName(IFNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(IFNAME))

			hwAddr := fmt.Sprintf("%s", link.Attrs().HardwareAddr)
			Expect(hwAddr).To(HavePrefix(hwaddr.PrivateMACPrefixString))

			// Ensure the default route
			routes, err := netlink.RouteList(link, 0)
			Expect(err).NotTo(HaveOccurred())

			var defaultRouteFound bool
			for _, route := range routes {
				defaultRouteFound = (route.Dst == nil && route.Src == nil && route.Gw.Equal(gwaddr))
				if defaultRouteFound {
					break
				}
			}
			Expect(defaultRouteFound).To(Equal(true))

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err := testutils.CmdDelWithResult(targetNs.Path(), IFNAME, func() error {
				return cmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Make sure macvlan link has been deleted
		err = targetNs.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			link, err := netlink.LinkByName(IFNAME)
			Expect(err).To(HaveOccurred())
			Expect(link).To(BeNil())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("ensure bridge address", func() {
		const IFNAME = "bridge0"
		const EXPECTED_IP = "10.0.0.0/8"
		const CHANGED_EXPECTED_IP = "10.1.2.3/16"

		conf := &NetConf{
			NetConf: types.NetConf{
				CNIVersion: "0.2.0",
				Name:       "testConfig",
				Type:       "bridge",
			},
			BrName: IFNAME,
			IsGW:   true,
			IPMasq: false,
			MTU:    5000,
		}

		gwnFirst := &net.IPNet{
			IP:   net.IPv4(10, 0, 0, 0),
			Mask: net.CIDRMask(8, 32),
		}

		gwnSecond := &net.IPNet{
			IP:   net.IPv4(10, 1, 2, 3),
			Mask: net.CIDRMask(16, 32),
		}

		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			bridge, err := setupBridge(conf)
			Expect(err).NotTo(HaveOccurred())
			// Check if ForceAddress has default value
			Expect(conf.ForceAddress).To(Equal(false))

			err = ensureBridgeAddr(bridge, gwnFirst, conf.ForceAddress)
			Expect(err).NotTo(HaveOccurred())

			//Check if IP address is set correctly
			addrs, err := netlink.AddrList(bridge, syscall.AF_INET)
			Expect(len(addrs)).To(Equal(1))
			addr := addrs[0].IPNet.String()
			Expect(addr).To(Equal(EXPECTED_IP))

			//The bridge IP address has been changed. Error expected when ForceAddress is set to false.
			err = ensureBridgeAddr(bridge, gwnSecond, false)
			Expect(err).To(HaveOccurred())

			//The IP address should stay the same.
			addrs, err = netlink.AddrList(bridge, syscall.AF_INET)
			Expect(len(addrs)).To(Equal(1))
			addr = addrs[0].IPNet.String()
			Expect(addr).To(Equal(EXPECTED_IP))

			//Reconfigure IP when ForceAddress is set to true and IP address has been changed.
			err = ensureBridgeAddr(bridge, gwnSecond, true)
			Expect(err).NotTo(HaveOccurred())

			//Retrieve the IP address after reconfiguration
			addrs, err = netlink.AddrList(bridge, syscall.AF_INET)
			Expect(len(addrs)).To(Equal(1))
			addr = addrs[0].IPNet.String()
			Expect(addr).To(Equal(CHANGED_EXPECTED_IP))

			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})
})
