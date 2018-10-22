package docs

const (
	KonkUse   string = `konk <command> <args>`
	KonkShort string = ``
	KonkLong  string = ``

	NymphUse   string = `nymph <subcommand> <args>`
	NymphShort string = `Operations over a node conducted by local nymph daemon`
	NymphLong  string = ``

	CoordinatorUse   string = `coordinator <args>`
	CoordinatorShort string = `Coordinator operations`
	CoordinatorLong  string = ``

	ConsoleUse   string = `console [flags] <command> <args>`
	ConsoleShort string = `Access coordinator over a console`
	ConsoleLong  string = ``

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

	CriuMigrateUse   string = `migrate`
	CriuMigrateShort string = `Migrate a process to another node using CRIU`
	CriuMigrateLong  string = ``

	CriuReceiveUse   string = `receive`
	CriuReceiveShort string = `Receive a process from another node using CRIU`
	CriuReceiveLong  string = ``

	InitUse   string = `init <args>`
	InitShort string = `Container initialization operations`
	InitLong  string = `Never use this program standalone`

	PrestartUse   string = `prestart <args>`
	PrestartShort string = `Container initialization operations`
	PrestartLong  string = `Never use this program standalone`
)
