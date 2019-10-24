package network

import (
	"fmt"
	"path"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/planetA/konk/config"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type NetworkOvs struct {
	client *ovs.Client
	bridge string
}

func NewOvs() (Network, error) {
	log.Trace("Init ovs network")
	// Check if driver are loaded
	client := ovs.New(
		ovs.Timeout(2))

	bridgeName := config.GetString(config.OvsBridgeName)
	// Create bridge
	if err := client.VSwitch.AddBridge(bridgeName); err != nil {
		log.Fatalf("failed to add bridge: %v", err)
		return nil, err
	}

	// Connect to other peers
	return &NetworkOvs{
		client: client,
		bridge: bridgeName,
	}, nil
}

func ovsPrestartHook(n *NetworkOvs, state *specs.State) error {
	netNsPath := path.Join("/proc", fmt.Sprintf("%v", state.Pid), "ns/net")
	log.WithFields(log.Fields{
		"state": state,
		"netns": netNsPath,
	}).Debug("Prestart hook")

	err := n.client.VSwitch.AddPort(n.bridge, "p0")
	if err != nil {
		log.Fatal(err)
		return err
	}

	if err := n.client.VSwitch.Set.Interface("p0", ovs.InterfaceOptions{
		Type: ovs.InterfaceTypeInternal,
	}); err != nil {
		log.Fatal(err)
		return err
	}

	port, err := netlink.LinkByName("p0")
	if err != nil {
		return fmt.Errorf("Failed to get port: %v", err)
	}

	addr, err := netlink.ParseAddr("169.254.169.254/32")
	if err != nil {
		log.Fatal(err)
		return err
	}

	if err := netlink.AddrAdd(port, addr); err != nil {
		log.Fatal(err)
		return err
	}

	if err := netlink.LinkSetUp(port); err != nil {
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
	n.client.VSwitch.DeletePort(n.bridge, "p0")
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
	n.client.VSwitch.DeleteBridge(n.bridge)
}
