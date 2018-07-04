package util

import (
	"log"

	"github.com/vishvananda/netlink"

)

func CreateBridge(name string) *netlink.Bridge {
	la := netlink.NewLinkAttrs()

	la.Name = name
	bridge := &netlink.Bridge{LinkAttrs: la}
	LinkAdd(bridge)

	return bridge
}

func CreateMacvlan(name string) *netlink.Macvlan {
	la := netlink.NewLinkAttrs()

	la.Name = name
	macvlan := &netlink.Macvlan{LinkAttrs: la}
	LinkAdd(macvlan)

	return macvlan
}

func CreateVethPair(id int) (netlink.Link, netlink.Link) {
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

	LinkAdd(veth)

	vethLink, err := netlink.LinkByName(vethNameId)
	if err != nil {
		log.Panicf("Can' get a veth link %s: %v", vethNameId, err)
	}

	vpeer, err := netlink.LinkByName(vpeerNameId)
	if err != nil {
		log.Panicf("Can't get a peer link %s: %v", vpeerNameId, err)
	}

	return vethLink, vpeer
}
