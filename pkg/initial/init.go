// +build linux cgo

package initial

// #include "init.h"
import "C"

import "fmt"

func Run(id int) (int, error) {
	initPid := C.run_init_process(C.int(id))
	if initPid == -1 {
		return -1, fmt.Errorf("Failed to launch init process")
	}

	return int(initPid), nil
}
