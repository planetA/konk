
#### Configure where the criu directory is
ifeq ($(shell hostname),jupiter)
	CRIU_DIR=$(HOME)/research/projects/ffmk/singularity-criu/criu
else
	CRIU_DIR=$(HOME)/singularity-criu/criu
endif


GOFILES=$(shell find . -name '[^.]*\.go')

all: konk

konk: $(GOFILES)
	go build -tags seccomp

install: konk
	go get

install-suid:
	-cp konk konk-suid
	-chown root:root konk-suid
	-chmod u+s konk-suid
	-cp /home/user/go/bin/konk /home/user/go/bin/konk-suid
	-chown root:root /home/user/go/bin/konk-suid
	-chmod u+s /home/user/go/bin/konk-suid

.PHONY: deploy
deploy: konk
	cp ./konk ./provisioning/roles/demo/files/
	vagrant up
