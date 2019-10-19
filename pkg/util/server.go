package util

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"time"

	"github.com/ugorji/go/codec"
)

// Create a network listener
func CreateListener(port int) (net.Listener, error) {
	address := fmt.Sprintf(":%v", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Could not open a port (%v): %v", address, err)
	}

	log.Printf("Creating a listener at at %v\n", address)

	return listener, err
}

func ServerLoop(listener net.Listener) error {
	var handle codec.MsgpackHandle

	for {
		// XXX: conn should be closed. But where and how? Right now this is a resource leak
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("Failed to open a connection: %v", err)
		}

		go func() {
			rpcCodec := codec.MsgpackSpecRpc.ServerCodec(conn, &handle)
			// XXX: doc says that I should run following line in a go statement
			// https://golang.org/pkg/net/rpc/#ServeConn
			rpc.ServeCodec(rpcCodec)
		}()
	}

	return nil
}

// Serve a connection
func Serve(listener net.Listener) (net.Conn, error) {
	handle := new(codec.MsgpackHandle)

	// XXX: see XXX above about closing the connection
	conn, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("Failed to open a connection: %v", err)
	}

	rpcCodec := codec.MsgpackSpecRpc.ServerCodec(conn, handle)
	rpc.ServeCodec(rpcCodec)

	return conn, nil
}

func DialRpcServerOnce(hostname string, port int) (*rpc.Client, error) {
	address := fmt.Sprintf("%v:%v", hostname, port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Cannot reach the server (%v): %v", address, err)
	}

	var handle codec.MsgpackHandle
	rpcCodec := codec.MsgpackSpecRpc.ClientCodec(conn, &handle)
	rpcClient := rpc.NewClientWithCodec(rpcCodec)

	return rpcClient, nil
}

func DialRpcServer(hostname string, port int) (*rpc.Client, error) {
	for {
		rpcClient, err := DialRpcServerOnce(hostname, port)
		if err == nil {
			return rpcClient, nil
		}

		log.Println("Failed to dial server: ", err)
		time.Sleep(5 * time.Second)
		log.Println("Trying to dial once again")
	}
}
