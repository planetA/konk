package util

import (
	"net"
)

var (
	VethName    string = "veth"
	VpeerName   string = "vpeer"
	EthName     string = "enp2s0"
	BridgeName  string = "br0"
	MacvlanName string = "macvlan0"
	DefaultMAC  string = "42:d6:7d:f7:3e:00"

	ContainerNet net.IPNet = net.IPNet{
	 	IP:   net.IPv4(172, 16, 0, 0),
		Mask: net.CIDRMask(16, 32),
	}
	ContainerBroadcast = net.IPv4(172, 16, 255, 255)

	DhclientPath string = "/sbin/dhclient"
	CriuPath     string = "/home/user/singularity-criu/install/sbin/criu"
	CriuImageDir string = "/tmp/criu.images"
)
