package container

import (
	"fmt"
	"log"

	"github.com/planetA/konk/pkg/util"

	"github.com/vishvananda/netlink"
)

type VethPair struct {
	veth  netlink.Link // Inner end
	vpeer netlink.Link // Outer end
}

func NewVethPair(id Id) (*VethPair, error) {
	vethNameId := GetDevName(util.VethName, id)
	vpeerNameId := GetDevName(util.VpeerName, id)

	// Set appropriate MAC address for the container interface
	hwAddr := CreateNewHardwareAddr(id)

	// Parameters to create a link
	vethTemplate := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:         vethNameId,
			HardwareAddr: hwAddr,
		},
		PeerName: vpeerNameId,
	}

	err := netlink.LinkAdd(vethTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to create veth pair: %v", err)
	}

	// Get the actually constructed link
	veth, err := netlink.LinkByName(vethNameId)
	if err != nil {
		return nil, fmt.Errorf("Can' get a veth link %s: %v", vethNameId, err)
	}

	vpeer, err := netlink.LinkByName(vpeerNameId)
	if err != nil {
		return nil, fmt.Errorf("Can't get a peer link %s: %v", vpeerNameId, err)
	}

	return &VethPair{
		veth:  veth,
		vpeer: vpeer,
	}, nil
}

func (v VethPair) Close() {
	if err := netlink.LinkDel(v.veth); err != nil {
		log.Println("Failed to delete veth: ", err)
	}

	// I'm not sure if this is even supposed to be ever deleted. Veth is never in the host
	// network namespace, meaning that this command is never supposed to work. Additionally,
	// if I delete vpeer, veth is supposed to be deleted automatically.
	//
	// Consider this is a safety measure, if veth stayed in the host network namespace, and
	// veth somehow was not deleted.
	if err := netlink.LinkDel(v.vpeer); err != nil {
		log.Println("Failed to delete vpeer: ", err)
	}
}
