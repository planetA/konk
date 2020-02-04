package network

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/util"

	"github.com/vishvananda/netlink"
	// "github.com/vishvananda/netns"
)

type Network interface {
	InstallHooks(config *configs.Config) error

	AddLabels(labels container.Labels) error

	// Add external devices for checkpointing
	DeclareExternal(rank container.Rank) []string

	// Uninitialize the network
	Destroy()

	// That is bad hack. No idea how to do it right.
	PostRestore(container *container.Container) error

	setNetworkType(networkType string)
}

type Hooks interface {
	Prestart(state *specs.State) error
	Poststart(state *specs.State) error
	Poststop(state *specs.State) error
}

type baseNetwork struct {
	networkType string
}

const (
	networkTypeOvs  = "ovs"
	networkTypeVeth = "veth"
)

const (
	hookTypePrestart  = "prestart"
	hookTypePoststart = "poststart"
	hookTypePoststop  = "poststop"
)

func (n *baseNetwork) createHook(hookType string) (configs.CommandHook, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return configs.CommandHook{}, err
	}

	args := []string{"konk",
		"hook",
		hookType,
		n.networkType}
	duration := time.Second * 3
	return configs.NewCommandHook(configs.Command{
		Path:    "/proc/self/exe",
		Args:    args,
		Env:     os.Environ(),
		Dir:     cwd,
		Timeout: &duration,
	}), nil
}

func (n *baseNetwork) setNetworkType(networkType string) {
	n.networkType = networkType
}

func readState() (*specs.State, error) {
	var state specs.State

	dec := json.NewDecoder(os.Stdin)
	err := dec.Decode(&state)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return &state, nil
}

func RunHook(hookType, networkType string) error {
	log.WithFields(log.Fields{
		"hook":    hookType,
		"network": networkType,
	}).Debug("Calling network hook")

	var hooks Hooks
	switch networkType {
	case networkTypeOvs:
		hooks = &hooksOvs{}
	case networkTypeVeth:
		hooks = &hooksVeth{}
	default:
		log.WithFields(log.Fields{
			"hook":    hookType,
			"network": networkType,
		}).Debug("Unknown network hook type")
		panic("Unknown network hook type")
	}

	var hook func(*specs.State) error

	switch hookType {
	case hookTypePrestart:
		hook = hooks.Prestart
	case hookTypePoststart:
		hook = hooks.Poststart
	case hookTypePoststop:
		hook = hooks.Poststop
	default:
		panic("Impossible hook type")
	}

	state, err := readState()
	if err != nil {
		return fmt.Errorf("Failed to read state: %v", err)
	}

	if err := hook(state); err != nil {
		log.WithError(err).Error("Failed running hook")
	}

	log.WithError(err).WithFields(log.Fields{
		"hook":    hookType,
		"network": networkType,
	}).Debug("Hook finished")

	return nil
}

func (n *baseNetwork) InstallHooks(config *configs.Config) error {
	hook, err := n.createHook(hookTypePrestart)
	if err != nil {
		return err
	}
	config.Hooks.Prestart = append(config.Hooks.Prestart, hook)

	hook, err = n.createHook(hookTypePoststart)
	if err != nil {
		return err
	}
	config.Hooks.Poststart = append(config.Hooks.Poststart, hook)

	hook, err = n.createHook(hookTypePoststop)
	if err != nil {
		return err
	}
	config.Hooks.Poststop = append(config.Hooks.Poststop, hook)

	return nil
}

func (n *baseNetwork) AddLabels(labels container.Labels) error {
	return nil
}

func New(networkType string) (net Network, err error) {
	switch networkType {
	case "ovs":
		net, err = NewOvs()
	case "veth":
		net, err = NewVeth()
	default:
		log.WithField("type", networkType).Panicf("Unknown network type")
		return nil, fmt.Errorf("Unknown network type")
	}

	if err != nil {
		log.WithError(err).WithField("type", networkType).Error("Failed to create network")
		return nil, err
	}

	net.setNetworkType(networkType)
	return net, nil
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

// func NewNetwork(rank Rank, path string) (error) {
// 	runtime.LockOSThread()
// 	defer runtime.UnlockOSThread()

// 	// First get the bridge
// 	bridge := getBridge(util.BridgeName)

// 	// Only then create anything
// 	namespace, err := attachNamespaceInit(path, Net)
// 	if err != nil {
// 		return err
// 	}
// 	defer namespace.Close()

// 	vethPair, err := NewVethPair(rank)
// 	if err != nil {
// 		return err
// 	}

// 	// Put end of the pair into corresponding namespaces
// 	if err := netlink.LinkSetNsFd(vethPair.veth, int(namespace.Guest)); err != nil {
// 		return fmt.Errorf("Could not set a namespace for %s: %v", vethPair.veth.Attrs().Name, err)
// 	}

// 	if err := netlink.LinkSetNsFd(vethPair.vpeer, int(namespace.Host)); err != nil {
// 		return fmt.Errorf("Could not set a namespace for %s: %v", vethPair.vpeer.Attrs().Name, err)
// 	}

// 	// Get handle to new namespace
// 	nsHandle, err := netlink.NewHandleAt(netns.NsHandle(namespace.Guest))
// 	if err != nil {
// 		return fmt.Errorf("Could not get a handle for namespace %v: %v", rank, err)
// 	}
// 	defer nsHandle.Delete()

// 	// Set slave-master relationships between bridge the physical interface
// 	netlink.LinkSetMaster(vethPair.vpeer, bridge)

// 	// Put links up
// 	if err := nsHandle.LinkSetUp(vethPair.veth); err != nil {
// 		return fmt.Errorf("Could not set interface %s up: %v", vethPair.veth.Attrs().Name, err)
// 	}
// 	if err := netlink.LinkSetUp(vethPair.vpeer); err != nil {
// 		return fmt.Errorf("Could not set interface %s up: %v", vethPair.vpeer.Attrs().Name, err)
// 	}
// 	nsHandle.AddrAdd(vethPair.veth, createContainerAddr(rank))

// 	lo, err := nsHandle.LinkByName("lo")
// 	if err != nil {
// 		return fmt.Errorf("Cannot acquire loopback: %v", err)
// 	}
// 	if err := nsHandle.LinkSetUp(lo); err != nil {
// 		return fmt.Errorf("Could not set interface %s up: %v", lo.Attrs().Name, err)
// 	}

// 	netDevs := make([]NetDev, 0)

// 	log.WithField("net_devs", append(netDevs, vethPair)).Trace("network")
// 	return nil
// }

// func (n *Network) Close() {
// 	log.Println("Closing network")
// for _, dev := range n.netDevs {
// 	dev.Close()
// }
// }
