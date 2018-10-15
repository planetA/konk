// +build !linux !cgo

package initial

import "C"

import log

func init() {
	log.Panic("Unsupported platform")
}
