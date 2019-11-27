package network

import (
	"fmt"
	"net"
	"os"

	"github.com/planetA/konk/config"
	log "github.com/sirupsen/logrus"
)

func getOtherPeers() ([]string, error) {
	peerNames := config.GetStringSlice(config.NymphNetworkPeers)
	if len(peerNames) < 2 {
		log.Debug("No peers to connect")
		return nil, fmt.Errorf("No peers to connect: %v", peerNames)
	}

	// TODO: check that hostname is in peerNames
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("failed to get hostname: %v", err)
		return nil, fmt.Errorf("failed to get hostname: %v", err)
	}

	log.WithFields(log.Fields{
		"peers": peerNames,
		"host":  hostname,
	}).Debug("Connecting bridges")

	otherPeers := []string{}
	for _, peer := range peerNames {
		if peer == hostname {
			continue
		}
		addr, err := net.LookupHost(peer)
		if err != nil {
			log.WithFields(log.Fields{
				"peer":  peer,
				"error": err,
			}).Fatal("Could not resolve peer name")
			return nil, err
		}
		if len(addr) < 1 {
			log.WithField("peer", peer).Fatal("Hostname did not resolve into IP")
			return nil, fmt.Errorf("Hostname did not resolve into IP: %v", peer)
		}

		log.WithFields(log.Fields{
			"peer":     peer,
			"all_addr": addr,
			"addr":     addr[0],
		}).Debug("Resolved peer name to IP")
		otherPeers = append(otherPeers, addr[0])
	}
	if len(otherPeers) < 1 || len(otherPeers) == len(peerNames) {
		log.WithFields(log.Fields{
			"peers":    peerNames,
			"hostname": hostname,
		}).Error("Peer list is wrong")
		return nil, fmt.Errorf("Peer list is wrong")
	}
	return otherPeers, nil
}
