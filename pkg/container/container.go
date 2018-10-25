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

	"golang.org/x/sys/unix"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/initial"
	"github.com/planetA/konk/pkg/util"
)

type Container struct {
	Id         Id
	Namespaces []Namespace
	Path       string
	Init       *InitProc
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

// Create a container with an init process inside
func NewContainerInit(id Id) (*Container, error) {
	var err error
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to create socket pair: %v", err)
	}

	outerSocket := os.NewFile(uintptr(fds[0]), "outer")
	innerSocket := os.NewFile(uintptr(fds[1]), "inner")
	defer innerSocket.Close()

	// In future, potentially, run will return the container path. For now we just construct it
	cmd, err := initial.Run(innerSocket)
	if err != nil {
		return nil, err
	}

	containerPath := fmt.Sprintf("%v/%v%v",
		config.GetString(config.ContainerRootDir),
		config.GetString(config.ContainerBaseName),
		id)

	initProc := newInitProc(containerPath, cmd, outerSocket)

	if err := initProc.sendParameters(id); err != nil {
		return nil, fmt.Errorf("Sending parameters failed: %v", err)
	}

	if err := initProc.waitInit(); err != nil {
		return nil, fmt.Errorf("Init process was not ready: %v", err)
	}

	return &Container{
		Id:   id,
		Init: initProc,
		Path: containerPath,
	}, nil
}

func (c *Container) ConfigureNetwork() error {

	// First get the bridge
	bridge := getBridge(util.BridgeName)

	// Only then create anything
	namespace, err := attachNamespaceInit(c.Path, Net)
	if err != nil {
		return err
	}
	defer namespace.Close()

	veth, vpeer, err := createVethPair(c.Id)
	if err != nil {
		return err
	}

	// Put end of the pair into corresponding namespaces
	if err := netlink.LinkSetNsFd(veth, int(namespace.Host)); err != nil {
		return fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	}

	if err := netlink.LinkSetNsFd(vpeer, int(namespace.Guest)); err != nil {
		return fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	}

	// Get handle to new namespace
	nsHandle, err := netlink.NewHandleAt(netns.NsHandle(namespace.Guest))
	if err != nil {
		return fmt.Errorf("Could not get a handle for namespace %v: %v", c.Id, err)
	}
	defer nsHandle.Delete()

	// Set slave-master relationships between bridge the physical interface
	netlink.LinkSetMaster(veth, bridge)

	// Put links up
	if err := nsHandle.LinkSetUp(vpeer); err != nil {
		return fmt.Errorf("Could not set interface %s up: %v", vpeer.Attrs().Name, err)
	}
	if err := netlink.LinkSetUp(veth); err != nil {
		return fmt.Errorf("Could not set interface %s up: %v", veth.Attrs().Name, err)
	}
	nsHandle.AddrAdd(vpeer, createContainerAddr(c.Id))

	lo, err := nsHandle.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("Cannot acquire loopback: %v", err)
	}
	if err := nsHandle.LinkSetUp(lo); err != nil {
		return fmt.Errorf("Could not set interface %s up: %v", lo.Attrs().Name, err)
	}

	return nil
}

func NewContainer(id Id) (*Container, error) {
	oldContainer, _ := newContainerClosed(id)
	oldContainer.Delete()

	// First get the bridge
	bridge := getBridge(util.BridgeName)

	// Only then create anything
	namespace, err := newNamespace(Net|Pid|Uts, id)
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
	defer nsHandle.Delete()

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
		Id:         id,
		Namespaces: []Namespace{*namespace},
	}, nil
}

func (c *Container) Notify() error {
	return c.Init.notify()
}

func (c *Container) Close() {
	c.Init.Close()
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

	for _, ns := range container.Namespaces {
		ns.Close()
	}

	return nil
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

func ContainerAttachPid(pid int) (*Container, error) {
	namespace, err := attachPidNamespace(Uts, pid)
	if err != nil {
		return nil, fmt.Errorf("Failed to attach to a container: %v", err)
	}

	return &Container{
		Id:         namespace.Id,
		Namespaces: []Namespace{*namespace},
	}, nil
}

// Attach to the container by the PID of the init process
func ContainerAttachInit(path string, nsTypesRequested Type) (*Container, error) {
	AllTypes := [...]Type{Uts, Ipc, User, Net, Pid, Mount}
	namespaces := make([]Namespace, 0, len(AllTypes))
	for _, nsTypeCur := range AllTypes {
		if (nsTypeCur & nsTypesRequested) == 0 {
			continue
		}
		curNamespace, err := attachNamespaceInit(path, nsTypeCur)
		if err != nil {
			return nil, fmt.Errorf("Failed to attach to a container: %v", err)
		}

		namespaces = append(namespaces, *curNamespace)
	}

	return &Container{
		Id:         namespaces[0].Id,
		Namespaces: namespaces,
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
func (container *Container) Activate(domainType DomainType) error {
	for _, ns := range container.Namespaces {
		if err := ns.Activate(domainType); err != nil {
			return fmt.Errorf("Cannot activate %v: %v", ns.TypeString(), err)
		}
	}
	return nil
}

func (container *Container) CloseOnExec(domainType DomainType) {
	for _, ns := range container.Namespaces {
		ns.CloseOnExec(domainType)
	}
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
