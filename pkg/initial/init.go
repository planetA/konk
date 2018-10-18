// +build linux cgo

package initial

// #cgo pkg-config: msgpack 
// #cgo CXXFLAGS: -std=c++17
// #cgo LDFLAGS: -lstdc++fs
// #include "init.h"
import "C"

import "fmt"

// Start the init process that is also the child of the nymph
func Run(socket int) (int, error) {
	initPid := C.run_init_process(C.int(socket))
	if initPid == -1 {
		return -1, fmt.Errorf("Failed to launch init process")
	}

	return int(initPid), nil
}
