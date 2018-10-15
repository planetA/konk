// Functions used by prestart

package container

import (
	"log"
	
	// "github.com/vishvananda/netlink"
)

func CreateNetwork(id int) error {
	log.Printf("Create network: %v\n", id)
	// veth, vpeer, err := createVethPair(Id(id))
	// if err != nil {
	// 	return err
	// }

	// // Put end of the pair into corresponding namespaces
	// if err := netlink.LinkSetNsFd(veth, int(network.Host)); err != nil {
	// 	return nil, fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	// }

	// if err := netlink.LinkSetNsFd(vpeer, int(network.Guest)); err != nil {
	// 	return nil, fmt.Errorf("Could not set a namespace for %s: %v", veth.Attrs().Name, err)
	// }

	return nil
}
