package node

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/vishvananda/netlink"

	"github.com/planetA/konk/pkg/util"
)

func launchDhcpClient(devName string) {
	cmd := exec.Command(util.DhclientPath, "-v", devName)
	err := cmd.Run()
	if err != nil {
		log.Panicf("Failed to launch dhcp client for device %s: %v", devName, err)
	}
}

func getIpv4(link netlink.Link) ([]netlink.Addr, error) {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("Could not get the list of addresses for link %v: %v", link, err)
	}

	if len(addrs) != 1 {
		log.Printf("More than one address found: %v\n", addrs)
	}

	return addrs, nil
}

func addrFlush(link netlink.Link) {
	addrs, _ := netlink.AddrList(link, netlink.FAMILY_ALL)

	for _, addr := range addrs {
		if err := netlink.AddrDel(link, &addr); err != nil {
			log.Print(err)
		}
	}
}

func generateInnerOmpi(addr netlink.Addr) netlink.Addr {
	base := util.ContainerNet

	base.IP = base.IP.To4()
	base.IP[3] = addr.IP[3]

	addr.IPNet = &base
	addr.Broadcast = util.ContainerBroadcast

	return addr
}

