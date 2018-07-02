package node

import (
	"fmt"
	"log"
	"bytes"
	"os/exec"

	"github.com/vishvananda/netlink"

	"github.com/planetA/konk/pkg/util"
)

var (
)

func createBridge(name string) *netlink.Bridge {
	la := netlink.NewLinkAttrs()

	la.Name = name
	bridge := &netlink.Bridge{LinkAttrs: la}
	util.LinkAdd(bridge)

	return bridge
}

func createMacvlan(name string) *netlink.Macvlan {
	la := netlink.NewLinkAttrs()

	la.Name = name
	macvlan := &netlink.Macvlan{LinkAttrs: la}
	util.LinkAdd(macvlan)

	return macvlan
}

func getEthernet(name string) netlink.Link {
	eth, err := netlink.LinkByName(name)
	if err != nil {
		log.Panic("Could not get %s: %v\n", name, err)
	}

	return eth
}

func launchDhcpClient(devName string) {
	cmd := exec.Command("dhclient", devName)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Panicf("Failed to launch dhcp client for device %s: %v", devName, err)
	}
}

/*
Here I create two bridges and connect a physical ethernet to one bridge.
*/
func Init(id int) {

	eth := getEthernet(util.EthName)
	bridge := createBridge(util.BridgeName)
	macvlan := createMacvlan(util.MacvlanName)

	// Set slave-master relationships between bridge and links

	netlink.LinkSetMaster(eth, bridge)
	netlink.LinkSetMaster(macvlan, bridge)

	// Set MAC addresses for macvlan and bridge

	oldAddr := eth.Attrs().HardwareAddr
	newAddr := util.ComputeNewHardwareAddr(oldAddr)

	netlink.LinkSetHardwareAddr(bridge, newAddr)
	netlink.LinkSetHardwareAddr(macvlan, oldAddr)

	netlink.LinkSetUp(bridge)
	netlink.LinkSetUp(macvlan)

	launchDhcpClient(bridge.Attrs().Name)
	launchDhcpClient(macvlan.Attrs().Name)

	fmt.Println("Initializing", id, newAddr)
}
