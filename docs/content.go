package docs

const (
	KonkUse   string = `konk <command> <args>`
	KonkShort string = ``
	KonkLong  string = ``

	NodeUse   string = `node <subcommand> <args>`
	NodeShort string = `Node scale operations`
	NodeLong  string = ``

	NodeInitUse   string = `init <id>`
	NodeInitShort string = `Init the node for running migratable MPI applications`
	NodeInitLong  string = ``

	ContainerUse   string = `container <id> <subcommand> <args>`
	ContainerShort string = `Operations on containers`
	ContainerLong  string = ``

	ContainerCreateUse   string = `create`
	ContainerCreateShort string = `Create a container with a prespecified id`
	ContainerCreateLong  string = ``

	ContainerDeleteUse   string = `delete`
	ContainerDeleteShort string = `Delete a container with a prespecified id`
	ContainerDeleteLong  string = ``

	ContainerRunUse   string = `run <env>`
	ContainerRunShort string = `Run a container with a prespecified id`
	ContainerRunLong  string = ``

	CriuUse   string = `criu <id> <subcommand> <args>`
	CriuShort string = `Interface to CRIU`
	CriuLong  string = ``

	CriuDumpUse   string = `dump`
	CriuDumpShort string = `Dump a process in a CRIU`
	CriuDumpLong  string = ``
)
