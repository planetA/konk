package container

import (
	"fmt"
	"log"
	"net"

	"github.com/planetA/konk/pkg/util"

	"github.com/vishvananda/netlink"
)

func GetDevName(devType string, rank Rank) string {
	return fmt.Sprintf("%s%v", devType, rank)
}

/*
In our network, I need to set the first byte of MAC to 42, to get ip addresses in a particular subnet.
*/
func ComputeNewHardwareAddr(oldAddr net.HardwareAddr) net.HardwareAddr {
	newAddr := oldAddr
	newAddr[0] = byte(0x42)
	return newAddr
}

/*
In our network, I need to set the first byte of MAC to 42, to get ip addresses in a particular subnet.
*/
func CreateNewHardwareAddr(rank Rank) net.HardwareAddr {
	newAddr, _ := net.ParseMAC(util.DefaultMAC)
	newAddr[0] = byte(0x42)
	newAddr[len(newAddr)-1] = byte(rank)
	return newAddr
}

func CreateContainerAddr(rank Rank) *netlink.Addr {
	base := util.ContainerNet

	base.IP = base.IP.To4()
	base.IP[2] = 1
	if rank > 253 {
		log.Panic("Unsupported container rank: %v", rank)
	}
	base.IP[3] = byte(rank + 1)

	return &netlink.Addr{
		IPNet: &base,
	}
}
