package network

import (
	"fmt"
	"os"
	"strings"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/planetA/konk/config"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"

	"github.com/vishvananda/netns"
)

type NetworkOvs struct {
	client    *ovs.Client
	bridge    string
	vxlanPort string
}

func (n *NetworkOvs) configurePeers() error {
	if err := n.client.VSwitch.AddPort(n.bridge, n.vxlanPort); err != nil {
		return err
	}

	otherPeers, err := getOtherPeers()
	if err != nil {
		return fmt.Errorf("Getting other peers failed: %v", err)
	}

	err = n.client.VSwitch.Set.Interface(n.vxlanPort, ovs.InterfaceOptions{
		Type:     ovs.InterfaceTypeVXLAN,
		RemoteIP: strings.Join(otherPeers, ","),
	})
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func NewOvs() (Network, error) {
	log.Trace("Init ovs network")
	// Check if driver are loaded
	client := ovs.New(
		ovs.Timeout(5),
		ovs.Debug(true),
	)

	bridgeName := config.GetString(config.OvsBridgeName)
	// Create bridge
	if err := client.VSwitch.AddBridge(bridgeName); err != nil {
		log.Fatalf("failed to add bridge: %v", err)
		return nil, err
	}

	ovs := &NetworkOvs{
		client:    client,
		bridge:    bridgeName,
		vxlanPort: "ovsvxlan0",
	}

	if err := ovs.configurePeers(); err != nil {
		return nil, err
	}

	// Connect to other peers
	return ovs, nil
}

func (n *NetworkOvs) cleanupPort(portName string) {
	log.WithFields(log.Fields{
		"bridge": n.bridge,
		"port":   portName,
	}).Trace("Deleting bridge port")
	n.client.VSwitch.DeletePort(n.bridge, portName)
}

func ovsPortName(state *specs.State) (string, error) {
	id, ok := state.Annotations["konk-id"]
	if !ok {
		return "", fmt.Errorf("Konk id is not set")
	}

	portName := fmt.Sprintf("p%v", id)
	return portName, nil
}

func ovsPortAddr(state *specs.State) (string, error) {
	addr, ok := state.Annotations["konk-ip"]
	if !ok {
		return "", fmt.Errorf("Konk ip is not set")
	}

	return addr, nil
}

func ovsPrestartHook(n *NetworkOvs, state *specs.State) error {
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

	portName, err := ovsPortName(state)
	if err != nil {
		log.Fatal(err)
		return err
	}

	addrString, err := ovsPortAddr(state)
	if err != nil {
		log.Fatal(err)
		return err
	}

	n.cleanupPort(portName)
	if err := n.client.VSwitch.AddPort(n.bridge, portName); err != nil {
		log.Fatal(err)
		return err
	}
	if err := n.client.VSwitch.Set.Interface(portName, ovs.InterfaceOptions{
		Type: ovs.InterfaceTypeInternal,
	}); err != nil {
		log.Fatal(err)
		return err
	}

	port, err := netlink.LinkByName(portName)
	if err != nil {
		return fmt.Errorf("Failed to get port: %v", err)
	}

	addr, err := netlink.ParseAddr(addrString)
	if err != nil {
		log.Fatal(err)
		return err
	}

	if err := netlink.LinkSetNsPid(port, state.Pid); err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"pid":   state.Pid,
		}).Fatal("Failed to get put link into net ns")
		return err
	}

	if err := handle.AddrAdd(port, addr); err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"port":  portName,
			"addr":  addr,
		}).Fatal("Adding address failed")
		return err
	}

	if err := handle.LinkSetUp(port); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func ovsPoststartHook(n *NetworkOvs, state *specs.State) error {
	log.Debug("Poststart hook")
	return nil
}

func ovsPoststopHook(n *NetworkOvs, state *specs.State) error {
	log.Debug("Poststop hook")

	portName, err := ovsPortName(state)
	if err != nil {
		return nil
	}

	ns, err := netns.GetFromPid(state.Pid)
	if err != nil {
		return nil
	}
	defer ns.Close()

	handle, err := netlink.NewHandleAt(ns)
	if err != nil {
		log.WithError(err).Debug("Getting handle from ns failed")
		return nil
	}
	defer handle.Delete()

	port, err := handle.LinkByName(portName)
	if err != nil {
		return nil
	}

	netlink.LinkSetNsPid(port, os.Getpid())

	n.cleanupPort(portName)
	return nil
}

type OvsHook func(*NetworkOvs, *specs.State) error

func NewOvsHook(network *NetworkOvs, hook OvsHook) configs.Hook {
	return configs.NewFunctionHook(func(state *specs.State) error {
		return hook(network, state)
	})
}

func (n *NetworkOvs) InstallHooks(config *configs.Config) error {
	config.Hooks.Prestart = append(config.Hooks.Prestart, NewOvsHook(n, ovsPrestartHook))
	config.Hooks.Poststart = append(config.Hooks.Poststart, NewOvsHook(n, ovsPoststartHook))
	config.Hooks.Poststop = append(config.Hooks.Poststop, NewOvsHook(n, ovsPoststopHook))

	return nil
}

func (n *NetworkOvs) Destroy() {
	n.cleanupPort(n.vxlanPort)
	n.client.VSwitch.DeleteBridge(n.bridge)
}
