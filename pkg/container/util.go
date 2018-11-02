package container

import (
	"fmt"
	"log"
	"net"

	"github.com/planetA/konk/pkg/util"

	"github.com/vishvananda/netlink"
)

func GetDevName(devType string, id Id) string {
	return fmt.Sprintf("%s%v", devType, id)
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
func CreateNewHardwareAddr(id Id) net.HardwareAddr {
	newAddr, _ := net.ParseMAC(util.DefaultMAC)
	newAddr[0] = byte(0x42)
	newAddr[len(newAddr)-1] = byte(id)
	return newAddr
}

func createContainerAddr(id Id) *netlink.Addr {
	base := util.ContainerNet

	base.IP = base.IP.To4()
	base.IP[2] = 1
	if id > 253 {
		log.Panic("Unsupported container id: %v", id)
	}
	base.IP[3] = byte(id + 1)

	return &netlink.Addr{
		IPNet: &base,
	}
}
