// Supporting package for the nymph daemon
package nymph

import (
	"fmt"
	"net/rpc"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/util"
)

// Client to connect to the nymph recipient daemon during migration
type MigrationClient struct {
	client *rpc.Client
	seq    int
}

// Create new connection to a nymph.
func NewMigrationClient(hostname string) (*MigrationClient, error) {
	port := config.GetInt(config.NymphPort)
	rpcClient, err := util.DialRpcServer(hostname, port)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"host": hostname,
		"port": port,
	}).Info("Connected to a nymph")

	return &MigrationClient{
		client: rpcClient,
	}, nil
}

func (m *MigrationClient) ImageInfo(rank container.Rank, id string) error {
	args := &ImageInfoArgs{rank, id}

	log.WithFields(log.Fields{
		"rank": rank,
		"id":   id,
	}).Debug("Send image info")

	var seq int
	err := m.client.Call(rpcImageInfo, args, &seq)
	if err != nil {
		return err
	}
	m.seq = seq

	return nil
}

func (m *MigrationClient) FileInfo(filename string, fileInfo os.FileInfo) error {
	args := &FileInfoArgs{
		Filename: filename,
		Size:     fileInfo.Size(),
		Mode:     fileInfo.Mode(),
		ModTime:  fileInfo.ModTime(),
	}

	log.WithFields(log.Fields{
		"File": filename,
		"Size": args.Size,
		"Mode": args.Mode,
		"ModTime": args.ModTime,
	}).Debug("Sending file info")

	var seq int
	err := m.client.Call(rpcFileInfo, args, &seq)
	if err != nil {
		return err
	}
	if seq != m.seq+1 {
		return fmt.Errorf("Unexpected sequence number: %v", seq)
	}
	m.seq = seq

	return nil
}

func (m *MigrationClient) Hello(say string) error {
	args := &HelloArgs{say}

	log.WithFields(log.Fields{
		"rpc": rpcHello,
		"say": say,
	}).Debug("Saying hello")

	var seq int
	err := m.client.Call(rpcHello, args, &seq)
	if err != nil {
		return err
	}

	return nil
}

func (c *MigrationClient) Close() {
	c.client.Close()
}
