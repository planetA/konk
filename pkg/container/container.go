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

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/planetA/konk/pkg/util"
)

type Container struct {
	Id      int
	Network *Namespace
	Pid     *Namespace
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

func newContainerClosed(id int) (*Container, error) {
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

func NewContainer(id int) (*Container, error) {
	oldContainer, _ := newContainerClosed(id)
	oldContainer.Delete()

	// First get the bridge
	bridge := getBridge(util.BridgeName)

	// Only then create anything
	network, err := newNamespace(Network, id)
	if err != nil {
		return nil, err
	}

	pid, err := newNamespace(Pid, id)
	if err != nil {
		return nil, err
	}

	veth, vpeer, err := util.CreateVethPair(id)
	if err != nil {
		return nil, err
	}

	// Put end of the pair into corresponding namespaces
	if err := netlink.LinkSetNsFd(veth, int(network.Host)); err != nil {
		return nil, fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	}

	if err := netlink.LinkSetNsFd(vpeer, int(network.Guest)); err != nil {
		return nil, fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	}

	// Get handle to new namespace
	nsHandle, err := netlink.NewHandleAt(netns.NsHandle(network.Guest))
	if err != nil {
		return nil, fmt.Errorf("Could not get a handle for namespace %s: %v", id, err)
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
	nsHandle.AddrAdd(vpeer, util.CreateContainerAddr(id))

	lo, err := nsHandle.LinkByName("lo")
	if err != nil {
		return nil, fmt.Errorf("Cannot acquire loopback: %v", err)
	}
	if err := nsHandle.LinkSetUp(lo); err != nil {
		return nil, fmt.Errorf("Could not set interface %s up: %v", lo.Attrs().Name, err)
	}

	return &Container{
		Id:      id,
		Network: network,
		Pid:     pid,
	}, nil
}

func (container *Container) Delete() error {
	// Container does not exist, hence nothing to delete
	if container == nil {
		return nil
	}

	// Delete namespace
	nsPath := util.GetNetNsPath(container.Id)

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
	vethId, err := netlink.LinkByName(util.GetNameId(util.VethName, container.Id))
	if err == nil {
		netlink.LinkDel(vethId)
	}

	vpeerId, err := netlink.LinkByName(util.GetNameId(util.VpeerName, container.Id))
	if err == nil {
		netlink.LinkDel(vpeerId)
	}

	container.Network.Close()

	return nil
}

func getContainerId(pid int) (int, error) {

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

		containerIdVarName := `OMPI_COMM_WORLD_RANK`
		if envVar == containerIdVarName {
			if len(tuple) > 1 {
				return strconv.Atoi(tuple[1])
			}
		}

		begin = i + 1
	}

	return -1, fmt.Errorf("Container ID variable is not found")
}

func ContainerAttachPid(pid int) (*Container, error) {
	networkNs, err := attachPidNamespace(Network, pid)
	if err != nil {
		return nil, fmt.Errorf("Failed to attach to a container: %v", err)
	}

	pidNs, err := attachPidNamespace(Pid, pid)
	if err != nil {
		return nil, fmt.Errorf("Failed to attach to a container: %v", err)
	}

	return &Container{
		Id:      networkNs.Id,
		Network: networkNs,
		Pid:     pidNs,
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
	container.Network.Activate(domainType)
	container.Pid.Activate(domainType)
}

func (container *Container) CloseOnExec(domainType DomainType) {
	container.Network.CloseOnExec(domainType)
	container.Pid.CloseOnExec(domainType)
}

func (container *Container) launchCommand(args []string) error {
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
		return fmt.Errorf("Application exited with an error: %v", err)
	}

	cmd.Wait()

	return nil
}

/*
Create a network namespace, a veth pair, put one end into the namespace and
another end connect to the bridge
*/
func Create(id int) error {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_, err := NewContainer(id)
	if err != nil {
		return fmt.Errorf("Failed to create container %d: %v", id, err)
	}

	return nil
}

func Delete(id int) {
	log.Printf("Deleting container with id %v", id)

	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	container, _ := newContainerClosed(id)
	if err := container.Delete(); err != nil {
		log.Printf("Could not delete the container")
	}
}

func Run(id int, args []string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ctx, cancel := util.NewContext()
	defer cancel()

	container, err := NewContainer(id)
	if err != nil {
		return fmt.Errorf("Failed to create a container: %v", err)
	}
	go func() {
		select {
		case <-ctx.Done():
			container.Delete()
		}
	}()

	err = container.launchCommand(args)
	if err != nil {
		return err
	}

	return nil
}
