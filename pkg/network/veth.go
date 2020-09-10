package network

import (
	"crypto/sha512"
	"fmt"
	"net"
	"os"
	"strconv"

	//"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/util"

	"github.com/planetA/konk/pkg/container"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	// "github.com/vishvananda/netns"
)

type VethPair struct {
	veth  netlink.Link // Inner end
	vpeer netlink.Link // Outer end
}

func NewVethPair(rank container.Rank) (*VethPair, error) {
	vethNameRank := container.GetDevName(util.VethName, rank)
	vpeerNameRank := container.GetDevName(util.VpeerName, rank)

	// Set appropriate MAC address for the container interface
	hwAddr := container.CreateNewHardwareAddr(rank)

	// Parameters to create a link
	vethTemplate := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:         vethNameRank,
			HardwareAddr: hwAddr,
		},
		PeerName: vpeerNameRank,
	}

	err := netlink.LinkAdd(vethTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to create veth pair: %v", err)
	}

	// Get the actually constructed link
	veth, err := netlink.LinkByName(vethNameRank)
	if err != nil {
		return nil, fmt.Errorf("Can' get a veth link %s: %v", vethNameRank, err)
	}

	vpeer, err := netlink.LinkByName(vpeerNameRank)
	if err != nil {
		return nil, fmt.Errorf("Can't get a peer link %s: %v", vpeerNameRank, err)
	}

	return &VethPair{
		veth:  veth,
		vpeer: vpeer,
	}, nil
}

// Delete veth pair. Notably, it is enough to delete only one end.
func DeleteVethPair(rank container.Rank) error {
	var peerErr, ethErr error

	vpeerNameRank := container.GetDevName(util.VpeerName, rank)
	vpeer, err := netlink.LinkByName(vpeerNameRank)
	if err == nil {
		if err := netlink.LinkDel(vpeer); err != nil {
			log.Println("Failed to delete vpeer: ", err)
		}
	} else {
		peerErr = err
	}

	vethNameRank := container.GetDevName(util.VethName, rank)
	veth, err := netlink.LinkByName(vethNameRank)
	if err == nil {
		if err := netlink.LinkDel(veth); err != nil {
			log.Println("Failed to delete vpeer: ", err)
		}
	} else {
		ethErr = err
	}

	if peerErr != nil && ethErr != nil {
		return fmt.Errorf("Failed to delete pair: vpeer: %v veth: %v", peerErr, ethErr)
	}

	return nil
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

type NetworkVeth struct {
	baseNetwork

	bridge *netlink.Bridge
	vxlan  *netlink.Vxlan
}

func createVxlan() (*netlink.Vxlan, error) {
	parentName := config.GetString(config.VethVxlanDev)
	parent, err := netlink.LinkByName(parentName)
	if err != nil {
		return nil, fmt.Errorf("No parent linke %v: %v", parentName, err)
	}

	la := netlink.NewLinkAttrs()
	la.Name = config.GetString(config.VethVxlanName)
	vxlan := &netlink.Vxlan{
		LinkAttrs:    la,
		Port:         config.GetInt(config.VethVxlanPort),
		VxlanId:      config.GetInt(config.VethVxlanId),
		Group:        config.GetIP(config.VethVxlanGroup),
		VtepDevIndex: parent.Attrs().Index,
	}

	// Remove stale link, if one exists
	netlink.LinkDel(vxlan)

	if err := netlink.LinkAdd(vxlan); err != nil {
		return nil, err
	}

	return vxlan, nil
}

const (
	BridgeHwaddrBase string = "c6:56:8e:a3:aa:3a"
)

func getBridgeHwaddr() (net.HardwareAddr, error) {
	// XXX: very hacky
	base, err := net.ParseMAC(BridgeHwaddrBase)
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	s := sha512.New512_256()
	s.Write([]byte(hostname))
	sum := s.Sum(nil)

	base[len(base)-1] = base[len(base)-1] + sum[len(sum)-1]
	base[len(base)-2] = base[len(base)-2] + sum[len(sum)-2]

	return base, nil
}

func createBridge() (*netlink.Bridge, error) {
	var err error

	la := netlink.NewLinkAttrs()
	la.Name = config.GetString(config.VethBridgeName)
	la.HardwareAddr, err = getBridgeHwaddr()
	if err != nil {
		return nil, err
	}

	bridge := &netlink.Bridge{LinkAttrs: la}

	// Remove stale link, if one exists
	netlink.LinkDel(bridge)

	if err := netlink.LinkAdd(bridge); err != nil {
		return nil, err
	}

	return bridge, nil
}

func NewVeth() (Network, error) {
	log.Trace("Init veth network")

	var err error
	net := &NetworkVeth{}
	defer func() {
		if err != nil {
			net.Destroy()
		}
	}()

	net.bridge, err = createBridge()
	if err != nil {
		return nil, fmt.Errorf("could not add bridge: %v\n", err)
	}

	net.vxlan, err = createVxlan()
	if err != nil {
		return nil, fmt.Errorf("could not add vxlan: %v\n", err)
	}

	if err = netlink.LinkSetMaster(net.vxlan, net.bridge); err != nil {
		return nil, fmt.Errorf("could not add vxlan to bridge: %v\n", err)
	}

	if err = netlink.LinkSetUp(net.vxlan); err != nil {
		return nil, fmt.Errorf("Failed to put vxlan up: %v", err)
	}

	if err = netlink.LinkSetUp(net.bridge); err != nil {
		return nil, fmt.Errorf("Failed to put bridge up: %v", err)
	}

	return net, nil
}

func vethPortRank(state *specs.State) (container.Rank, error) {
	rank, ok := state.Annotations["konk-rank"]
	if !ok {
		return -1, fmt.Errorf("Konk rank is not set")
	}

	val, err := strconv.Atoi(rank)
	if err != nil {
		return -1, fmt.Errorf("Invalid rank: v", rank)
	}

	return container.Rank(val), nil
}

func vethPortAddr(state *specs.State) (string, error) {
	addr, ok := state.Annotations["konk-ip"]
	if !ok {
		return "", fmt.Errorf("Konk ip is not set")
	}

	return addr, nil
}

func vethBridgeName(state *specs.State) (string, error) {
	bridge, ok := state.Annotations["konk-bridge"]
	if !ok {
		return "", fmt.Errorf("Konk bridge is not set")
	}

	return bridge, nil
}

type hooksVeth struct {
	baseHooks
}

func (h *hooksVeth) Prestart(state *specs.State) error {
	log.WithField("state", state).Debug("Prestart")

	if state.Status != "creating" {
		// nothing to do here
		return nil
	}

	ns, err := netns.GetFromPid(state.Pid)
	if err != nil {
		log.WithError(err).Fatal("Getting ns from PID failed")
		return err
	}
	defer ns.Close()

	handle, err := netlink.NewHandleAt(ns)
	if err != nil {
		log.WithError(err).Fatal("Getting handle from ns failed")
		return err
	}
	defer handle.Delete()

	rank, err := vethPortRank(state)
	if err != nil {
		log.WithError(err).Fatal("port-rank")
		return err
	}

	addrString, err := vethPortAddr(state)
	if err != nil {
		log.Fatal(err)
		return err
	}

	log.WithFields(log.Fields{
		"rank": rank,
		"addr": addrString,
	}).Debug("Creating veth pair")

	pair, err := NewVethPair(rank)
	if err != nil {
		return err
	}

	addr, err := netlink.ParseAddr(addrString)
	if err != nil {
		log.Fatal(err)
		return err
	}

	// Put end of the pair into corresponding namespaces
	if err := netlink.LinkSetNsPid(pair.veth, state.Pid); err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"pid":   state.Pid,
		}).Fatal("Failed to get put link into net ns")
		return err
	}

	if err := handle.AddrAdd(pair.veth, addr); err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"rank":  rank,
			"addr":  addr,
		}).Fatal("Adding address failed")
		return err
	}

	if err := handle.LinkSetUp(pair.veth); err != nil {
		log.Fatal(err)
		return err
	}

	bridgeName, err := vethBridgeName(state)
	if err != nil {
		log.WithError(err).Fatal("Bridge name failed")
		return err
	}
	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		log.WithError(err).Fatal("Did not find bridge")
		return err
	}
	// Set slave-master relationships between bridge the physical interface
	if err := netlink.LinkSetMaster(pair.vpeer, bridge.(*netlink.Bridge)); err != nil {
		log.WithError(err).Fatal("Failed to set master for peer")
		return err
	}

	if err := netlink.LinkSetUp(pair.vpeer); err != nil {
		return fmt.Errorf("Could not set interface %s up: %v", pair.vpeer.Attrs().Name, err)
	}

	return nil
}

func (h *hooksVeth) Poststop(state *specs.State) error {
	log.Debug("Poststop hook")

	rank, err := vethPortRank(state)
	if err != nil {
		log.WithError(err).Debug("port-rank")
		return nil
	}

	err = DeleteVethPair(rank)
	if err != nil {
		log.WithError(err).Error("Pair has not been found")
		return err
	}

	return fmt.Errorf("Success")
}

func destroyLink(link netlink.Link) {
	if link == nil {
		log.Debug("Device does not exist. Not deleted.")
		return
	}
	log.WithField("link", link).Debug("Destroying link deleted.")

	err := netlink.LinkDel(link)
	if err != nil {
		log.WithError(err).Warn("Destroying brdige failed")
	}
}

func (n *NetworkVeth) AddLabels(labels container.Labels) error {
	return labels.AddLabel("bridge", n.bridge.Name)
}

func (n *NetworkVeth) DeclareExternal(rank container.Rank) ([]string, bool) {
	return []string{fmt.Sprintf("veth[veth%v]:vpeer%v", rank, rank)}, true
}

func (n *NetworkVeth) PostRestore(cont *container.Container) error {
	vpeerNameRank := container.GetDevName(util.VpeerName, cont.Rank())
	vpeer, err := netlink.LinkByName(vpeerNameRank)
	if err != nil {
		return fmt.Errorf("Can't get a peer link %s: %v", vpeerNameRank, err)
	}

	// Set slave-master relationships between bridge the physical interface
	if err := netlink.LinkSetMaster(vpeer, n.bridge); err != nil {
		log.WithError(err).Fatal("Failed to set master for peer")
		return err
	}

	if err := netlink.LinkSetUp(vpeer); err != nil {
		return fmt.Errorf("Could not set interface %s up: %v", vpeer.Attrs().Name, err)
	}

	return nil
}

func (n *NetworkVeth) Destroy() {
	if n.vxlan != nil {
		destroyLink(n.vxlan)
	}

	if n.bridge != nil {
		destroyLink(n.bridge)
	}
}
