
#### Configure where the criu directory is
ifeq ($(shell hostname),jupiter)
	CRIU_DIR=$(HOME)/research/projects/ffmk/singularity-criu/criu
else
	CRIU_DIR=$(HOME)/singularity-criu/criu
endif


GOFILES=$(shell find . -name '[^.]*\.go')

KONK_PROTO_DIR=pkg/konk
KONK_PROTO_FILE=$(KONK_PROTO_DIR)/konk.proto
KONK_PROTO=$(KONK_PROTO_DIR)/konk.pb.go

all: konk

$(KONK_PROTO): $(KONK_PROTO_FILE)
	protoc --go_out=plugins=grpc:$(shell dirname $@) -I$(shell dirname $^) $^

konk: $(KONK_PROTO) $(GOFILES)
	go get
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
