package network

import (
	"fmt"
	"os"
	"strconv"

	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/container"
	log "github.com/sirupsen/logrus"
	// "github.com/vishvananda/netns"
)

const (
	lastQpnPath = "/proc/sys/net/rdma_rxe/last_qpn"
	lastMrnPath = "/proc/sys/net/rdma_rxe/last_mrn"
)

type NetworkRxe struct {
	baseNetwork
	qpnpn  uint
	minqpn uint
	mrnpn  uint
	minmrn uint
}

func NewRxe() (Network, error) {
	log.Trace("Init rxe network")

	var err error

	f, err := os.Open(lastQpnPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open last_qpn file: %v", err)
	}

	// Test read
	buf := make([]byte, 20)
	n, err := f.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("Failed to read last qpn file: %v", err)
	}

	_, err = strconv.Atoi(strings.TrimSpace(string(buf[:n])))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse last qpn file: %v", err)
	}

	// Connect to other peers
	return &NetworkRxe{
		qpnpn:  config.GetUint(config.RxeQpnpn),
		minqpn: config.GetUint(config.RxeMinqpn),
		mrnpn:  config.GetUint(config.RxeMrnpn),
		minmrn: config.GetUint(config.RxeMinmrn),
	}, nil
}

type hooksRxe struct {
	baseHooks
}

func writeParameter(path string, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("Failed to open last_qpn file: %v", err)
	}
	defer f.Close()

	_, err = f.WriteString(value)
	if err != nil {
		return fmt.Errorf("Failed to read last qpn file: %v", err)
	}

	return nil
}

func (h *hooksRxe) Prestart(state *specs.State) error {
	qpnStart, ok := state.Annotations["konk-rxe-qpnstart"]
	if !ok {
		return fmt.Errorf("Konk rank is not set")
	}

	if err := writeParameter(lastQpnPath, qpnStart); err != nil {
		return err
	}

	mrnStart, ok := state.Annotations["konk-rxe-mrnstart"]
	if !ok {
		return fmt.Errorf("Konk rank is not set")
	}

	if err := writeParameter(lastMrnPath, mrnStart); err != nil {
		return err
	}

	return nil
}

func (n *NetworkRxe) AddLabels(labels container.Labels) error {
	id, err := labels.GetLabelUint("nymph-id")
	if err != nil {
		return err
	}

	qpnStart := id*n.qpnpn + n.minqpn
	if err := labels.AddLabel("rxe-qpnstart", qpnStart); err != nil {
		return err
	}

	mrnStart := id*n.mrnpn + n.minmrn
	if err := labels.AddLabel("rxe-mrnstart", mrnStart); err != nil {
		return err
	}

	return nil
}
