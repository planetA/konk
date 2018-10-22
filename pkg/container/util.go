package container

import (
	"fmt"
	"log"
	"net"

	"github.com/planetA/konk/pkg/util"

	"github.com/vishvananda/netlink"
)

func createVethPair(id Id) (netlink.Link, netlink.Link, error) {
	vethNameId := getDevName(util.VethName, id)
	vpeerNameId := getDevName(util.VpeerName, id)

	// Set appropriate MAC address for the container interface
	hwAddr := CreateNewHardwareAddr(id)

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:         vethNameId,
			HardwareAddr: hwAddr,
		},
		PeerName: vpeerNameId,
	}

	err := netlink.LinkAdd(veth)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create veth pair: %v", err)
	}

	vethLink, err := netlink.LinkByName(vethNameId)
	if err != nil {
		return nil, nil, fmt.Errorf("Can' get a veth link %s: %v", vethNameId, err)
	}

	vpeer, err := netlink.LinkByName(vpeerNameId)
	if err != nil {
		return nil, nil, fmt.Errorf("Can't get a peer link %s: %v", vpeerNameId, err)
	}

	return vethLink, vpeer, nil
}

func getNetNsPath(id Id) string {
	return fmt.Sprintf("/var/run/netns/%s", getNameId(Net, id))
}

func getDevName(devType string, id Id) string {
	return fmt.Sprintf("%s%v", devType, id)
}

/*
In our network, I need to set the first byte of MAC to 42, to get ip addresses in a particular subnet.
*/
func ComputeNewHardwareAddr(oldAddr net.HardwareAddr) net.HardwareAddr {
	newAddr := oldAddr
	newAddr[0] = byte(0x42)
	return newAddr
}

/*
In our network, I need to set the first byte of MAC to 42, to get ip addresses in a particular subnet.
*/
func CreateNewHardwareAddr(id Id) net.HardwareAddr {
	newAddr, _ := net.ParseMAC(util.DefaultMAC)
	newAddr[0] = byte(0x42)
	newAddr[len(newAddr)-1] = byte(id)
	return newAddr
}

func createContainerAddr(id Id) *netlink.Addr {
	base := util.ContainerNet

	base.IP = base.IP.To4()
	base.IP[2] = 1
	if id > 253 {
		log.Panic("Unsupported container id: %v", id)
	}
	base.IP[3] = byte(id + 1)

	return &netlink.Addr{
		IPNet: &base,
	}
}
