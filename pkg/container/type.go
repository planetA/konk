package container

import (
	"golang.org/x/sys/unix"
)

type Type int

const (
	Uts Type = 1 << iota
	Ipc
	User
	Net
	Pid
	Mount
)

var namespaceNames = map[Type]string{
	Uts:   "uts",
	Ipc:   "ipc",
	User:  "user",
	Net:   "net",
	Pid:   "pid",
	Mount: "mnt",
}

var namespaceCodes = map[Type]int{
	Uts:   unix.CLONE_NEWUTS,
	Ipc:   unix.CLONE_NEWIPC,
	User:  unix.CLONE_NEWUSER,
	Net:   unix.CLONE_NEWNET,
	Pid:   unix.CLONE_NEWPID,
	Mount: unix.CLONE_NEWNS,
}

type Rank int
