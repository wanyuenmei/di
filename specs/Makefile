.PHONY: spark \
    	etcd \
	zookeeper

all: spark \
	zookeeper

REPO = quilt
DOCKER = docker

# ALL
etcd:
	cd etcd && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build run.go \
	&& ${DOCKER} build -t ${REPO}/etcd .

spark:
	cd spark && ${DOCKER} build -t ${REPO}/spark .

zookeeper:
	cd zookeeper && ${DOCKER} build -t ${REPO}/zookeeper .

# Include all .mk files so you can have your own local configurations
include $(wildcard *.mk)
