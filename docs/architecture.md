# Migration architecture

The migration workflow works as follows.

1. MigrationClient and MigrationServer connect

	Two instances of konk started with different parameters need to
    establish a connection over a gRPC protocol to transfer a
    checkpoint data.

2. MigrationClient prepares a checkpoint

	MigrationClient starts CRIU and tells it to dump a particular
    process

3. MigrationClient transfers the checkpoint to the MigrationServer

	The client sends all the data and tells the server to launch the
    recover from the checkpoint. Important is that before finishing
    the communication the client kills local container, because
    otherwise the communication between client and server
    breaks. Figuring out the root cause is not the most urgent issue,
    so I just left the XXX note in the log.

4. MigrationServer restores the checkpoint

	The server also starts CRIU, but uses it for recovering the
    checkpoint.

5. Both client and server cleanup temporary data

Right now I'm in the process of refactoring. The idea is to split the
code into following classes.

1. MigrationClient

	Manages the CriuService and Container classes. Talks to the
    MigrationServer over the network

2. MigrationServer

	Manages the CriuService and Container classes. Talks to the
    MigrationClient over the network

3. CriuService

	Interface to CRIU Service running as a separate process,
    communication goes over a UNIX socket.

4. Container encapsulates entities that belong to the "container"

	Right now a "container" include network namespace and the
    processes running within the network namespace


Schematically interaction looks as follows.

	+-----------------------+  Net   +-----------------------+
	|        MigCli         |<======>|        MigServ        |
	|                       |		 |                       |
	| +------+ +----------+ |		 | +------+ +----------+ |
	| | Cont | | CriuServ | |		 | | Cont | | CriuServ | |
	| +------+ +----------+ |		 | +------+ +----------+ |
	+-----------------^-----+		 +-----------------^-----+
                      |                                |
					  V                                V
				  +------+                         +------+
				  | CRIU |						   | CRIU |
				  +------+						   +------+
