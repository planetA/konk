package container

import (
	"fmt"

	"golang.org/x/sys/unix"
	"os"
	"runtime"
	"syscall"
)

type Namespace struct {
	Id    int
	Host  Handle
	Guest Handle
	Type  Type
}

// Compose a namespace name, give type and id
func getNameId(nsType Type, id int) string {
	return fmt.Sprintf("%s%v", namespaceNames[nsType], id)
}

// Compose a path to the namespace of a current task (thread) for a given namespace type
func getNsPath(nsType Type) string {
	return fmt.Sprintf("/proc/%d/task/%d/ns/%s", os.Getpid(), syscall.Gettid(), namespaceNames[nsType])
}

// Compose a path to the namespace of a process for a given namespace type and process ID
func getNsPathTask(nsType Type, pid int) string {
	return fmt.Sprintf("/proc/%d/task/%d/ns/%s", pid, pid, namespaceNames[nsType])
}

// Return a path to directory with named namespaces. If direcotry does not exist, create it.
func getNsDir(nsType Type) string {
	netNsDir := fmt.Sprintf("/var/run/%sns", namespaceNames[nsType])

	// Create netns directory if does not exist
	if _, err := os.Stat(netNsDir); os.IsNotExist(err) {
		os.Mkdir(netNsDir, os.ModePerm)
	}
	return netNsDir
}

// Create new namespace of a given type
func newNamespace(nsType Type, id int) (*Namespace, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	nsNameId := getNameId(nsType, id)
	nsDir := getNsDir(nsType)

	hostNs, err := openNamespace(nsType)
	if err != nil {
		return nil, fmt.Errorf("Failed to open host namespace: %v", err)
	}

	guestNs, err := createNamespace(nsType)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a namespace %s: %v", nsNameId, err)
	}

	// Mount newly created namespace where we want
	guestNsPath := fmt.Sprintf("%s/%s", nsDir, nsNameId)

	// Create a file to do mounting
	os.OpenFile(guestNsPath, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0666)

	nsPath := getNsPath(nsType)
	err = syscall.Mount(nsPath, guestNsPath, "", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to create a named namespace %s (%s): %v", nsNameId, guestNsPath, err)
	}

	namespace := &Namespace{
		Id:    id,
		Host:  hostNs,
		Guest: guestNs,
		Type:  nsType,
	}

	namespace.Activate(HostDomain)

	return namespace, nil

}

func attachPidNamespace(nsType Type, pid int) (*Namespace, error) {
	hostNs, err := openNamespace(nsType)
	if err != nil {
		return nil, fmt.Errorf("Could not get host network namespace: %v", err)
	}

	guestNs, err := openNamespacePid(nsType, pid)
	if err != nil {
		return nil, fmt.Errorf("Could not get network namespace for process %v: %v", pid, err)
	}

	id, err := getContainerId(pid)
	if err != nil {
		return nil, fmt.Errorf("Could not get container id for pid %v: %v", pid, err)
	}

	return &Namespace{
		Id:    id,
		Host:  hostNs,
		Guest: guestNs,
		Type:  nsType,
	}, nil
}

func (namespace *Namespace) Close() error {
	// Nothing to close
	if namespace == nil {
		return nil
	}

	if err := namespace.Host.Close(); err != nil {
		return err
	}
	if err := namespace.Guest.Close(); err != nil {
		return err
	}

	namespace = nil
	return nil

}

func (namespace *Namespace) getHandle(domainType DomainType) Handle {
	switch domainType {
	case GuestDomain:
		return namespace.Guest
	case HostDomain:
		return namespace.Host
	default:
		panic("Impossible domain type")
	}
}

func (namespace *Namespace) Activate(domainType DomainType) error {
	curNs := int(namespace.getHandle(domainType))

	err := unix.Setns(curNs, namespaceCodes[namespace.Type])
	if err != nil {
		return err
	}

	return nil
}

func (namespace *Namespace) CloseOnExec(domainType DomainType) {
	namespace.getHandle(domainType).CloseOnExec()
}