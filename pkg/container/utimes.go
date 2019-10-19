// +build linux,amd64

// This file provides a hacky implementation of chtimes with flags, as default one does not exist
package container

import (
	"os"
	"syscall"
	"time"
	"unsafe"
	"fmt"
)

const (
	_AT_FDCWD = -0x64
)

func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	default:
		return fmt.Errorf("Some error code %v", e.Error())
	}
}

func utimensat(fd int, path string, ts []syscall.Timespec, flags int) (err error) {
	_p1, err := syscall.BytePtrFromString(path)
	if err != nil {
		return
	}

	ts_uint := uintptr(unsafe.Pointer(&ts[0]))
	_, _, e1 := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(int(fd)), uintptr(unsafe.Pointer(_p1)), ts_uint, uintptr(flags), 0, 0)

	if e1 != 0 {
		return errnoErr(e1)
	}

	return
}

func UtimesNanoFlags(path string, ts []syscall.Timespec, flags int) (err error) {
	if len(ts) != 2 {
		return syscall.EINVAL
	}

	err = utimensat(_AT_FDCWD, path, ts, flags)

	return
}

// Chtimes changes the access and modification times of the named
// file, similar to the Unix utime() or utimes() functions.
//
// The underlying filesystem may truncate or round the values to a
// less precise time unit.
// If there is an error, it will be of type *PathError.
func ChtimesFlags(name string, atime time.Time, mtime time.Time, flags int) error {
	var utimes [2]syscall.Timespec
	utimes[0] = syscall.NsecToTimespec(atime.UnixNano())
	utimes[1] = syscall.NsecToTimespec(mtime.UnixNano())
	if e := UtimesNanoFlags(name, utimes[0:], flags); e != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: e}
	}
	return nil
}
