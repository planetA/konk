package util

import (
	"fmt"
	"log"
	"net"

	"github.com/vishvananda/netlink"
)

var (
	VethName    string = "veth"
	VpeerName   string = "vpeer"
	NsName      string = "net"
	EthName     string = "enp2s0"
	BridgeName  string = "br0"
	MacvlanName string = "macvlan0"
	DefaultMAC  string = "42:d6:7d:f7:3e:00"

	ContainerNet net.IPNet = net.IPNet{
		IP:   net.IPv4(172, 16, 0, 0),
		Mask: net.CIDRMask(16, 32),
	}
)

func LinkAdd(link netlink.Link) {
	err := netlink.LinkAdd(link)
	if err != nil {
		log.Panicf("Failed to add %s: %v\n", link.Attrs().Name, err)
	}
}

func GetNetNsPath(id int) string {
	return fmt.Sprintf("/var/run/netns/%s", GetNameId(NsName, id))
}

func GetNameId(nsName string, id int) string {
	return fmt.Sprintf("%s%v", nsName, id)
}

/*
In our network, I need to set the first byte of MAC to 42, to get ip addresses in a particular subnet.
*/
func ComputeNewHardwareAddr(oldAddr net.HardwareAddr) net.HardwareAddr {
	newAddr := oldAddr
	newAddr[0] = byte(0x42)
	fmt.Printf("%v -> %v\n", oldAddr[0], newAddr[0])
	return newAddr
}

/*
In our network, I need to set the first byte of MAC to 42, to get ip addresses in a particular subnet.
*/
func CreateNewHardwareAddr(id int) net.HardwareAddr {
	newAddr, _ := net.ParseMAC(DefaultMAC)
	newAddr[0] = byte(0x42)
	newAddr[len(newAddr)-1] = byte(id)
	return newAddr
}
