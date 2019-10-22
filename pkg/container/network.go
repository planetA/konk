package container

import (
	"fmt"
	"runtime"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/util"
	"github.com/planetA/konk/config"

	"github.com/opencontainers/runc/libcontainer/configs"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type Network struct {
	netDevs []NetDev
}

func getBridge(bridgeName string) *netlink.Bridge {
	bridgeLink, err := netlink.LinkByName(util.BridgeName)
	if err != nil {
		log.Panicf("Could not get %s: %v\n", util.BridgeName, err)
	}

	return &netlink.Bridge{
		LinkAttrs: *bridgeLink.Attrs(),
	}
}

func NewNetwork(id Id, path string) (*Network, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// First get the bridge
	bridge := getBridge(util.BridgeName)

	// Only then create anything
	namespace, err := attachNamespaceInit(path, Net)
	if err != nil {
		return nil, err
	}
	defer namespace.Close()

	vethPair, err := NewVethPair(id)
	if err != nil {
		return nil, err
	}

	// Put end of the pair into corresponding namespaces
	if err := netlink.LinkSetNsFd(vethPair.veth, int(namespace.Guest)); err != nil {
		return nil, fmt.Errorf("Could not set a namespace for %s: %v", vethPair.veth.Attrs().Name, err)
	}

	if err := netlink.LinkSetNsFd(vethPair.vpeer, int(namespace.Host)); err != nil {
		return nil, fmt.Errorf("Could not set a namespace for %s: %v", vethPair.vpeer.Attrs().Name, err)
	}

	// Get handle to new namespace
	nsHandle, err := netlink.NewHandleAt(netns.NsHandle(namespace.Guest))
	if err != nil {
		return nil, fmt.Errorf("Could not get a handle for namespace %v: %v", id, err)
	}
	defer nsHandle.Delete()

	// Set slave-master relationships between bridge the physical interface
	netlink.LinkSetMaster(vethPair.vpeer, bridge)

	// Put links up
	if err := nsHandle.LinkSetUp(vethPair.veth); err != nil {
		return nil, fmt.Errorf("Could not set interface %s up: %v", vethPair.veth.Attrs().Name, err)
	}
	if err := netlink.LinkSetUp(vethPair.vpeer); err != nil {
		return nil, fmt.Errorf("Could not set interface %s up: %v", vethPair.vpeer.Attrs().Name, err)
	}
	nsHandle.AddrAdd(vethPair.veth, createContainerAddr(id))

	lo, err := nsHandle.LinkByName("lo")
	if err != nil {
		return nil, fmt.Errorf("Cannot acquire loopback: %v", err)
	}
	if err := nsHandle.LinkSetUp(lo); err != nil {
		return nil, fmt.Errorf("Could not set interface %s up: %v", lo.Attrs().Name, err)
	}

	netDevs := make([]NetDev, 0)
	return &Network{
		netDevs: append(netDevs, vethPair),
	}, nil
}

func (n *Network) Close() {
	log.Println("Closing devices")
	for _, dev := range n.netDevs {
		dev.Close()
	}
}

func configureNetworkOvs(containerConfig *configs.Config) error {
	return nil
}

func configureNetwork(containerConfig *configs.Config) error {
	networkType := config.GetString(config.NymphNetwork)

	switch networkType {
	case "ovs":
		return configureNetworkOvs(containerConfig)
	default:
		log.WithField("type", networkType).Panicf("Unknown network type")
		panic("Unknown network type")
	}
}
