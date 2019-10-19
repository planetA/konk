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

	MpirunUse   string = `mpirun <image> <program> <args>`
	MpirunShort string = `Wrapper for the mpirun command`
	MpirunLong  string = ``

	RunUse   string = `run <args>`
	RunShort string = `Run a container with a prespecified id`
	RunLong  string = ``

	InitUse   string = `init <args>`
	InitShort string = `Container initialization operations`
	InitLong  string = `Never use this program standalone`
)
