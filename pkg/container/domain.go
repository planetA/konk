package container

// Type of the resource visibility. Possible domains are guest and host
type DomainType int

const (
	GuestDomain DomainType = iota
	HostDomain
)

