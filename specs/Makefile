.PHONY: spark \
	zookeeper

all: spark \
	zookeeper

REPO = quay.io/netsys
DOCKER = docker

# ALL

spark: build-spark push-spark

zookeeper: build-zookeeper push-zookeeper

# BUILD

build-spark:
	cd spark && ${DOCKER} build -t ${REPO}/spark .

build-zookeeper:
	cd zookeeper && ${DOCKER} build -t ${REPO}/zookeeper .

# PUSH

push-spark:
	${DOCKER} push ${REPO}/spark

push-zookeeper:
	${DOCKER} push ${REPO}/zookeeper

# Include all .mk files so you can have your own local configurations
include $(wildcard *.mk)
