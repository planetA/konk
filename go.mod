module github.com/planetA/konk

go 1.13

replace (
	github.com/godbus/dbus => github.com/godbus/dbus/v5 v5.0.3
	github.com/opencontainers/runc => github.com/planeta/runc mplaneta-external
)

require (
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/checkpoint-restore/go-criu v0.0.0-20191125063657-fcdcd07065c5
	github.com/containerd/console v0.0.0-20191206165004-02ecf6a7291e
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/digitalocean/go-openvswitch v0.0.0-20191122155805-8ce3b4218729
	github.com/docker/go-units v0.4.0 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/golang/protobuf v1.3.1
	github.com/mrunalp/fileutils v0.0.0-20171103030105-7d4729fb3618 // indirect
	github.com/opencontainers/runc v1.0.0-rc9.0.20191206223258-201b06374548
	github.com/opencontainers/runtime-spec v1.0.2-0.20191007145322-19e92ca81777
	github.com/opencontainers/selinux v1.3.0 // indirect
	github.com/seccomp/libseccomp-golang v0.9.1 // indirect
	github.com/shirou/gopsutil v2.19.11+incompatible
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.1
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	github.com/ugorji/go/codec v1.1.7
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df
	github.com/willf/bitset v1.1.10
	golang.org/x/sys v0.0.0-20191210023423-ac6580df4449
)
