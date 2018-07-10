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

	return addr
}

/*
Here I create two bridges and connect a physical ethernet to one bridge.
*/
func Init(id int) error {
	eth, err := netlink.LinkByName(util.EthName)
	if err != nil {
		return fmt.Errorf("Could not get %s: %v\n", util.EthName, err)
	}


	eth_ip, err := getIpv4(eth)
	if err != nil {
		return err
	}
	if len(eth_ip) < 1 {
		return fmt.Errorf("Expected the device %s to have at least one address", util.EthName)
	}

	ip_host := eth_ip[0]
	ip_host.Label = "br0"
	
	ip_inner := generateInnerOmpi(ip_host)
	log.Printf("Prepraing addrs: %s & %s", ip_host, ip_inner)

	addrFlush(eth)
	
	bridge := util.CreateBridge(util.BridgeName)

	// Set slave-master relationships between bridge the physical interface
	netlink.LinkSetMaster(eth, bridge)

	netlink.LinkSetUp(bridge)
	netlink.LinkSetUp(eth)

	// The order is important. This way OpenMPI will pick the
	// inner address first
	netlink.AddrAdd(bridge, &ip_inner)
	netlink.AddrAdd(bridge, &ip_host)
	// launchDhcpClient(bridge.Attrs().Name)

	fmt.Println("Initializing", id)

	return nil
}
