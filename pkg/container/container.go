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
	Id    int
	Host  netns.NsHandle
	Guest netns.NsHandle
}

func createNs(id int) (netns.NsHandle, netns.NsHandle) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oldNs, _ := netns.Get()

	newNs, err := netns.New()
	if err != nil {
		log.Panicf("Can't create a namespace %s%v: %v", util.NsName, id, err)
	}

	// Mount newly created namespace where we want

	// Create netns directory
	netNsDir := "/var/run/netns"
	if _, err := os.Stat(netNsDir); os.IsNotExist(err) {
		os.Mkdir(netNsDir, os.ModePerm)
	}

	// Create a file to do mounting
	nsNameId := util.GetNameId(util.NsName, id)
	newNsPath := fmt.Sprintf("%s/%s", netNsDir, nsNameId)
	os.OpenFile(newNsPath, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0666)

	nsPath := fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), syscall.Gettid())
	err = syscall.Mount(nsPath, newNsPath, "", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		log.Panicf("Can't create a named namespace %s (%s): %v", nsNameId, newNsPath, err)
	}

	netns.Set(oldNs)

	return newNs, oldNs
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
		Id:    id,
		Host:  -1,
		Guest: -1,
	}, nil
}

func NewContainer(id int) (*Container, error) {
	oldContainer, _ := newContainerClosed(id)
	oldContainer.Delete()

	// First get the bridge
	bridge := getBridge(util.BridgeName)

	// Only then create anything
	newNs, oldNs := createNs(id)

	veth, vpeer, err := util.CreateVethPair(id)
	if err != nil {
		return nil, err
	}

	// Put end of the pair into corresponding namespaces
	if err := netlink.LinkSetNsFd(veth, int(oldNs)); err != nil {
		return nil, fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	}

	if err := netlink.LinkSetNsFd(vpeer, int(newNs)); err != nil {
		return nil, fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	}

	// Get handle to new namespace
	nsHandle, err := netlink.NewHandleAt(newNs)
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
		Id:    id,
		Host:  oldNs,
		Guest: newNs,
	}, nil
}

func (container *Container) Delete() error {
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

	container.Host.Close()
	container.Guest.Close()

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
	hostNs, err := netns.Get()
	if err != nil {
		return nil, fmt.Errorf("Could not get host network namespace: %v", err)
	}

	guestNs, err := netns.GetFromPid(pid)
	if err != nil {
		return nil, fmt.Errorf("Could not get network namespace for process %v: %v", pid, err)
	}

	id, err := getContainerId(pid)
	if err != nil {
		return nil, fmt.Errorf("Could not get container id for pid %v: %v", pid, err)
	}

	return &Container{
		Id:    id,
		Host:  hostNs,
		Guest: guestNs,
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

func (container *Container) launchCommand(args []string) error {
	netns.Set(container.Guest)
	defer netns.Set(container.Host)
	syscall.CloseOnExec(int(container.Host))

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

	ctx := util.NewContext()

	container, err := NewContainer(id)
	if err != nil {
		return fmt.Errorf("Failed to create a container: %v", err)
	}
	go func() {
		select {
		case <-ctx.Done():
			err := container.Delete()
		}
	}()

	err = container.launchCommand(args)
	if err != nil {
		return err
	}

	return nil
}
