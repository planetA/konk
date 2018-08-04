package util

import (
	"fmt"

	"github.com/vishvananda/netlink"

)

func CreateBridge(name string) (*netlink.Bridge, error) {
	la := netlink.NewLinkAttrs()

	la.Name = name
	bridge := &netlink.Bridge{LinkAttrs: la}
	err := netlink.LinkAdd(bridge)
	if err != nil {
		return nil, fmt.Errorf("Failed to create bridge: %v", err)
	}

	return bridge, nil
}

func CreateMacvlan(name string) (*netlink.Macvlan, error) {
	la := netlink.NewLinkAttrs()

	la.Name = name
	macvlan := &netlink.Macvlan{LinkAttrs: la}
	err := netlink.LinkAdd(macvlan)
	if err != nil {
		return nil, fmt.Errorf("Failed to create macvlan: %v", err)
	}

	return macvlan, nil
}

func CreateVethPair(id int) (netlink.Link, netlink.Link, error) {
	vethNameId := GetNameId(VethName, id)
	vpeerNameId := GetNameId(VpeerName, id)

	// Set appropriate MAC address for the container interface
	hwAddr := CreateNewHardwareAddr(id)
	
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: vethNameId,
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
