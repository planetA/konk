package container

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/viper"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/util"
)

type Container struct {
	Id        Id
	Namespace *Namespace
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

func newContainerClosed(id Id) (*Container, error) {
	return &Container{
		Id: id,
	}, nil
}

func printAllLinks() {
	index := 1
	for {
		link, err := netlink.LinkByIndex(index)
		if index > 10 {
			break
		}

		if err == nil {
			fmt.Println(index, link)
		}
		index = index + 1
	}
}

func NewContainer(id Id) (*Container, error) {
	oldContainer, _ := newContainerClosed(id)
	oldContainer.Delete()

	// First get the bridge
	bridge := getBridge(util.BridgeName)

	// Only then create anything
	namespace, err := newNamespace(Network|Pid|Uts, id)
	if err != nil {
		return nil, err
	}

	veth, vpeer, err := createVethPair(id)
	if err != nil {
		return nil, err
	}

	// Put end of the pair into corresponding namespaces
	if err := netlink.LinkSetNsFd(veth, int(namespace.Host)); err != nil {
		return nil, fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	}

	if err := netlink.LinkSetNsFd(vpeer, int(namespace.Guest)); err != nil {
		return nil, fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	}

	// Get handle to new namespace
	nsHandle, err := netlink.NewHandleAt(netns.NsHandle(namespace.Guest))
	if err != nil {
		return nil, fmt.Errorf("Could not get a handle for namespace %v: %v", id, err)
	}

	// Set slave-master relationships between bridge the physical interface
	netlink.LinkSetMaster(veth, bridge)

	// Put links up
	if err := nsHandle.LinkSetUp(vpeer); err != nil {
		return nil, fmt.Errorf("Could not set interface %s up: %v", vpeer.Attrs().Name, err)
	}
	if err := netlink.LinkSetUp(veth); err != nil {
		return nil, fmt.Errorf("Could not set interface %s up: %v", veth.Attrs().Name, err)
	}
	nsHandle.AddrAdd(vpeer, createContainerAddr(id))

	lo, err := nsHandle.LinkByName("lo")
	if err != nil {
		return nil, fmt.Errorf("Cannot acquire loopback: %v", err)
	}
	if err := nsHandle.LinkSetUp(lo); err != nil {
		return nil, fmt.Errorf("Could not set interface %s up: %v", lo.Attrs().Name, err)
	}

	return &Container{
		Id:        id,
		Namespace: namespace,
	}, nil
}

func (container *Container) Delete() error {
	// Container does not exist, hence nothing to delete
	if container == nil {
		return nil
	}

	// Delete namespace
	nsPath := getNetNsPath(container.Id)

	if err := syscall.Unmount(nsPath, syscall.MNT_DETACH); err != nil {
		if err == syscall.ENOENT {
			return nil
		}
		return fmt.Errorf("Could not unmount the container %s: %v", nsPath, err)
	}

	if err := syscall.Unlink(nsPath); err != nil {
		return fmt.Errorf("Could not delete the container %s: %v", nsPath, err)
	}

	/* Theoretically we should not kill these two, because deleting a namespace should delete veth the ns contains. Deleting a veth should delete vpeer. But we delete everything to be on the safe side. */
	vethId, err := netlink.LinkByName(getDevName(util.VethName, container.Id))
	if err == nil {
		netlink.LinkDel(vethId)
	}

	vpeerId, err := netlink.LinkByName(getDevName(util.VpeerName, container.Id))
	if err == nil {
		netlink.LinkDel(vpeerId)
	}

	container.Namespace.Close()

	return nil
}

func getContainerId(pid int) (Id, error) {
	containerIdVarName := viper.GetString(config.ViperContainerIdEnv)

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

func ContainerAttachPid(pid int) (*Container, error) {
	namespace, err := attachPidNamespace(Network|Pid|Uts, pid)
	if err != nil {
		return nil, fmt.Errorf("Failed to attach to a container: %v", err)
	}

	return &Container{
		Id:        namespace.Id,
		Namespace: namespace,
	}, nil
}

// Attach to the container by the PID of the init process
func ContainerAttachInit(path string) (*Container, error) {
	namespace, err := attachNamespaceInit(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to attach to a container: %v", err)
	}

	return &Container{
		Id:        namespace.Id,
		Namespace: namespace,
	}, nil
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
func (container *Container) Activate(domainType DomainType) {
	container.Namespace.Activate(domainType)
}

func (container *Container) CloseOnExec(domainType DomainType) {
	container.Namespace.CloseOnExec(domainType)
}

func (container *Container) LaunchCommand(args []string) (*exec.Cmd, error) {
	container.Activate(GuestDomain)
	defer container.Activate(HostDomain)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: getCredential(),
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()

	if err != nil {
		return nil, fmt.Errorf("Application exited with an error: %v", err)
	}

	return cmd, nil
}

/*
Create a network namespace, a veth pair, put one end into the namespace and
another end connect to the bridge
*/
func Create(id Id) error {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_, err := NewContainer(id)
	if err != nil {
		return fmt.Errorf("Failed to create container %d: %v", id, err)
	}

	return nil
}

func Delete(id Id) {
	log.Printf("Deleting container with id %v", id)

	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	container, _ := newContainerClosed(id)
	if err := container.Delete(); err != nil {
		log.Printf("Could not delete the container")
	}
}
