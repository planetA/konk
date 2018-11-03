// +build go1.10

// We depend on go1.10, because of the behaviour of LockOSThread behaviour.
// See: https://github.com/vishvananda/netns/issues/17

package container

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/vishvananda/netlink"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/util"
)

type Container struct {
	Id         Id
	Namespaces []Namespace
	Path       string
	Init       *InitProc
	Network    *Network
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

// Create a container with an init process inside
func NewContainerInit(id Id) (*Container, error) {
	var err error
	initProc, err := NewInitProc(id)
	if err != nil {
		return nil, err
	}

	if err := initProc.sendParameters(id); err != nil {
		return nil, fmt.Errorf("Sending parameters failed: %v", err)
	}

	if err := initProc.waitInit(); err != nil {
		return nil, fmt.Errorf("Init process was not ready: %v", err)
	}

	containerPath := fmt.Sprintf("%v/%v%v",
		config.GetString(config.ContainerRootDir),
		config.GetString(config.ContainerBaseName),
		id)

	return &Container{
		Id:   id,
		Init: initProc,
		Path: containerPath,
	}, nil
}

func (c *Container) Notify() error {
	return c.Init.notify()
}

func (c *Container) Close() {
	for _, ns := range c.Namespaces {
		ns.Close()
	}

	c.Init.Close()

	c.Network.Close()

	// Delete container directory
	os.RemoveAll(c.Path)
}

func getContainerId(pid int) (Id, error) {
	containerIdVarName := config.GetString(config.ContainerIdEnv)

	environPath := fmt.Sprintf("/proc/%d/environ", pid)

	data, err := ioutil.ReadFile(environPath)
	if err != nil {
		return -1, err
	}

	begin := 0
	for i, char := range data {
		if char != 0 {
			continue
		}
		tuple := strings.Split(string(data[begin:i]), "=")
		envVar := tuple[0]

		if envVar == containerIdVarName && len(tuple) > 1 {
			id, err := strconv.Atoi(tuple[1])
			if err != nil {
				return -1, nil
			}
			return Id(id), nil
		}

		begin = i + 1
	}

	return -1, fmt.Errorf("Container ID variable is not found")
}

func getCredential() *syscall.Credential {
	grp, _ := os.Getgroups()
	grp32 := func(b []int) []uint32 {
		data := make([]uint32, len(b))
		for i, v := range b {
			data[i] = uint32(v)
		}
		return data
	}(grp)

	return &syscall.Credential{
		Uid:    uint32(os.Getuid()),
		Gid:    uint32(os.Getgid()),
		Groups: grp32,
	}

}

// Make host or guest container active
func (container *Container) Activate(domainType DomainType) error {
	for _, ns := range container.Namespaces {
		if err := ns.Activate(domainType); err != nil {
			return fmt.Errorf("Cannot activate %v: %v", ns.TypeString(), err)
		}
	}
	return nil
}

func LaunchCommandInitProc(initProc int, args []string) (*exec.Cmd, error) {
	launcherPath := config.GetString(config.KonkSysLauncher)
	cmd := exec.Command(launcherPath, args...)
	cmd.Env = append(os.Environ(),
		"KONK_INIT_PROC="+strconv.Itoa(initProc))

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("Application exited with an error: %v", err)
	}

	return cmd, nil
}
