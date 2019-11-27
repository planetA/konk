package network

import (
	"fmt"
	"strconv"

	//"strings"

	"github.com/opencontainers/runc/libcontainer/configs"
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

func NewVethPair(id container.Id) (*VethPair, error) {
	vethNameId := container.GetDevName(util.VethName, id)
	vpeerNameId := container.GetDevName(util.VpeerName, id)

	// Set appropriate MAC address for the container interface
	hwAddr := container.CreateNewHardwareAddr(id)

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

type NetworkVeth struct {
	bridge *netlink.Bridge
	vxlan  *netlink.Vxlan
	pairs  map[container.Id]*VethPair
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

	if err := netlink.LinkAdd(vxlan); err != nil {
		return nil, err
	}

	return vxlan, nil
}

func createBridge() (*netlink.Bridge, error) {
	la := netlink.NewLinkAttrs()
	la.Name = config.GetString(config.VethBridgeName)
	bridge := &netlink.Bridge{LinkAttrs: la}
	err := netlink.LinkAdd(bridge)
	if err != nil {
		return nil, err
	}

	return bridge, nil
}

func NewVeth() (Network, error) {
	log.Trace("Init veth network")

	var err error
	net := &NetworkVeth{
		pairs: make(map[container.Id]*VethPair),
	}
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

	// Connect to other peers
	return net, nil
}

func vethPortId(state *specs.State) (container.Id, error) {
	id, ok := state.Annotations["konk-id"]
	if !ok {
		return -1, fmt.Errorf("Konk id is not set")
	}

	val, err := strconv.Atoi(id)
	if err != nil {
		return -1, fmt.Errorf("Invalid id: v", id)
	}

	return container.Id(val), nil
}

func vethPortAddr(state *specs.State) (string, error) {
	addr, ok := state.Annotations["konk-ip"]
	if !ok {
		return "", fmt.Errorf("Konk ip is not set")
	}

	return addr, nil
}

func vethPrestartHook(n *NetworkVeth, state *specs.State) error {
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

	id, err := vethPortId(state)
	if err != nil {
		log.WithError(err).Fatal("port-id")
		return err
	}

	addrString, err := vethPortAddr(state)
	if err != nil {
		log.Fatal(err)
		return err
	}

	pair, err := NewVethPair(id)
	if err != nil {
		return err
	}

	_, ok := n.pairs[id]
	if ok {
		log.Fatal("Unexpected pair found")
	}
	n.pairs[id] = pair

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
			"id":    id,
			"addr":  addr,
		}).Fatal("Adding address failed")
		return err
	}

	if err := handle.LinkSetUp(pair.veth); err != nil {
		log.Fatal(err)
		return err
	}

	// Set slave-master relationships between bridge the physical interface
	if err := netlink.LinkSetMaster(pair.vpeer, n.bridge); err != nil {
		log.WithError(err).Fatal("Failed to set master for peer")
		return err
	}

	if err := netlink.LinkSetUp(pair.vpeer); err != nil {
		return fmt.Errorf("Could not set interface %s up: %v", pair.vpeer.Attrs().Name, err)
	}

	return nil
}

func vethPoststartHook(n *NetworkVeth, state *specs.State) error {
	log.Debug("Poststart hook")
	return nil
}

func vethPoststopHook(n *NetworkVeth, state *specs.State) error {
	log.Debug("Poststop hook")

	id, err := vethPortId(state)
	if err != nil {
		log.WithError(err).Debug("port-id")
		return nil
	}

	pair, ok := n.pairs[id]
	if !ok {
		log.Debug("Pair has not been found")
		return nil
	}

	pair.Close()
	return nil
}

type VethHook func(*NetworkVeth, *specs.State) error

func NewVethHook(network *NetworkVeth, hook VethHook) configs.Hook {
	return configs.NewFunctionHook(func(state *specs.State) error {
		return hook(network, state)
	})
}

func (n *NetworkVeth) InstallHooks(config *configs.Config) error {
	config.Hooks.Prestart = append(config.Hooks.Prestart, NewVethHook(n, vethPrestartHook))
	config.Hooks.Poststart = append(config.Hooks.Poststart, NewVethHook(n, vethPoststartHook))
	config.Hooks.Poststop = append(config.Hooks.Poststop, NewVethHook(n, vethPoststopHook))

	return nil
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

func (n *NetworkVeth) Destroy() {
	if n.vxlan != nil {
		destroyLink(n.vxlan)
	}

	if n.bridge != nil {
		destroyLink(n.bridge)
	}
}
