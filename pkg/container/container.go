package container

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/planetA/konk/pkg/util"
)

func createVethPair(id int) (netlink.Link, netlink.Link) {
	vethNameId := util.GetNameId(util.VethName, id)
	vpeerNameId := util.GetNameId(util.VpeerName, id)

	// Set appropriate MAC address for the container interface
	hwAddr := util.CreateNewHardwareAddr(id)
	fmt.Println(hwAddr)
	
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: vethNameId,
			HardwareAddr: hwAddr,
		},
		PeerName: vpeerNameId,
	}

	util.LinkAdd(veth)

	vethLink, err := netlink.LinkByName(vethNameId)
	if err != nil {
		log.Panicf("Can' get a veth link %s: %v", vethNameId, err)
	}

	vpeer, err := netlink.LinkByName(vpeerNameId)
	if err != nil {
		log.Panicf("Can't get a peer link %s: %v", vpeerNameId, err)
	}

	return vethLink, vpeer
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

func deleteNs(id int) {
	nsPath := util.GetNetNsPath(id)

	if err := syscall.Unmount(nsPath, syscall.MNT_DETACH); err != nil {
		log.Printf("Could not delete the container %s: %v", nsPath, err)
	}
}

/*
Create a network namespace, a veth pair, put one end into the namespace and
another end connect to the bridge
*/
func Create(id int) {
	log.Printf("Creating container with id %v", id)

	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	newNs, oldNs := createNs(id)

	veth, vpeer := createVethPair(id)
	if err := netlink.LinkSetNsFd(veth, int(newNs)); err != nil {
		log.Panic(err)
	}

	if err := netlink.LinkSetNsFd(vpeer, int(oldNs)); err != nil {
		log.Panic(err)
	}

	nsHandle, err := netlink.NewHandleAt(newNs)
	if err != nil {
		log.Panic(err)
	}
	
	if err := nsHandle.LinkSetUp(veth); err != nil {
		log.Panicf("Could not set interface %s up: %v", veth.Attrs().Name, err)
	}
	netlink.LinkSetUp(vpeer)

	ifaces, _ := net.Interfaces()
	fmt.Printf("Interfaces: %v\n", ifaces)
}

func Delete(id int) {
	log.Printf("Deleting container with id %v", id)

	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	deleteNs(id)

	/* Theoretically we should not kill these two, because deleting a namespace should delete veth the ns contains. Deleting a veth should delete vpeer. But we delete everything to be on the safe side. */
	vethId, err := netlink.LinkByName(util.GetNameId(util.NsName, id))
	if err == nil {
		netlink.LinkDel(vethId)
	}

	vpeerId, err := netlink.LinkByName(util.GetNameId(util.NsName, id))
	if err == nil {
		netlink.LinkDel(vpeerId)
	}
}
