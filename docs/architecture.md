# Overall architecture

Konk consists of two services:

1. Nymph

	A node-local daemon that manages all local containers and with the
    help from coordinator organises container migration between
    different nodes. Additionally, nymph should be responsible for
    managing NMD (node monitoring daemon) and MOSIX module.
	
2. Coordinator

	Coordinator is responsible for maintaining a global database of
    container locations. The coordinator receives the information from
    nymphs. The coordinator may organise migration of a container by
    first asking a nymph to prepare to receive a container, then,
    asking another nymph to send one. A client can connect to the
    coordinator and ask it to execute some commands.
    
Nymph uses runc libcontainer runtime to manage containers. Own implementation of
containers is not used anymore.
	
# Directory structure

1. srv

	Contains logic for running the three services: coproc, nymph and
    coordinator

2. pkg

	Supplementary packages needed by various services

3. cmd

	Command line interface to execute various konk commands
	
4. config

	Default values for configuration variables.

5. docs

	Documentation related package

# Migration architecture

~ATTENTION~: Deffinetelly outdated info. Now migration is done by libcontainer.

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
