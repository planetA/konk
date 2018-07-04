package container

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/planetA/konk/pkg/util"
)

type Container struct {
	Host  netns.NsHandle
	Guest netns.NsHandle
}

func createNs(id int) (netns.NsHandle, netns.NsHandle) {
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

	err = syscall.Mount("/proc/self/ns/net", newNsPath, "", syscall.MS_BIND|syscall.MS_REC, "")
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

func createContainer(id int) (*Container, error) {
	// First get the bridge
	bridge := getBridge(util.BridgeName)

	// Only then create anything
	newNs, oldNs := createNs(id)

	veth, vpeer := util.CreateVethPair(id)
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

	return &Container{
		Host:  oldNs,
		Guest: newNs,
	}, nil
}

func deleteContainer(id int) error {

	// Delete namespace
	nsPath := util.GetNetNsPath(id)

	if err := syscall.Unmount(nsPath, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("Could not delete the container %s: %v", nsPath, err)
	}

	/* Theoretically we should not kill these two, because deleting a namespace should delete veth the ns contains. Deleting a veth should delete vpeer. But we delete everything to be on the safe side. */
	vethId, err := netlink.LinkByName(util.GetNameId(util.VethName, id))
	if err == nil {
		netlink.LinkDel(vethId)
	}

	vpeerId, err := netlink.LinkByName(util.GetNameId(util.VpeerName, id))
	if err == nil {
		netlink.LinkDel(vpeerId)
	}

	return nil
}

/*
Create a network namespace, a veth pair, put one end into the namespace and
another end connect to the bridge
*/
func Create(id int) {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if _, err := createContainer(id); err != nil {
		log.Panic(err)
	}
}

func Delete(id int) {
	log.Printf("Deleting container with id %v", id)

	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := deleteContainer(id); err != nil {
		log.Panic(err)
	}
}

func Run(id int, args []string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := deleteContainer(id); err != nil {
		log.Println(err)
	}

	container, err := createContainer(id)
	if err != nil {
		log.Panic(err)
	}

	netns.Set(container.Guest)
	defer netns.Set(container.Host)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(os.Getuid()),
		},
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		log.Panicf("Failed to run the application: %v", err)
	}
}
