
#### Configure where the criu directory is
ifeq ($(shell hostname),jupiter)
	CRIU_DIR=$(HOME)/research/projects/ffmk/singularity-criu/criu
else
	CRIU_DIR=$(HOME)/singularity-criu/criu
endif


GOFILES=$(shell find . -name '[^.]*\.go')
RPC_PROTO_FILE=$(CRIU_DIR)/images/rpc.proto
RPC_PROTO_DIR=pkg/rpc
RPC_PROTO=$(RPC_PROTO_DIR)/rpc.pb.go

all: konk

$(RPC_PROTO): $(RPC_PROTO_FILE)
	mkdir -p $(shell dirname $@)
	protoc --go_out=$(shell dirname $@) -I$(shell dirname $^) $^

konk: $(RPC_PROTO) $(GOFILES)
	go build

install: konk
	go get

install-suid:
	-cp konk konk-suid
	-chown root:root konk-suid
	-chmod u+s konk-suid
	-cp /home/user/go/bin/konk /home/user/go/bin/konk-suid
	-chown root:root /home/user/go/bin/konk-suid
	-chmod u+s /home/user/go/bin/konk-suid
