package node

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func createBridge(name string) (*netlink.Bridge, error) {
	la := netlink.NewLinkAttrs()

	la.Name = name
	bridge := &netlink.Bridge{LinkAttrs: la}
	err := netlink.LinkAdd(bridge)
	if err != nil {
		return nil, fmt.Errorf("Failed to create bridge: %v", err)
	}

	return bridge, nil
}

func createMacvlan(name string) (*netlink.Macvlan, error) {
	la := netlink.NewLinkAttrs()

	la.Name = name
	macvlan := &netlink.Macvlan{LinkAttrs: la}
	err := netlink.LinkAdd(macvlan)
	if err != nil {
		return nil, fmt.Errorf("Failed to create macvlan: %v", err)
	}

	return macvlan, nil
}
