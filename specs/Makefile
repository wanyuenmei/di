.PHONY: spark \
	zookeeper

all: spark \
	zookeeper

REPO = quilt
DOCKER = docker

# ALL

spark:
	cd spark && ${DOCKER} build -t ${REPO}/spark .

zookeeper:
	cd zookeeper && ${DOCKER} build -t ${REPO}/zookeeper .

# Include all .mk files so you can have your own local configurations
include $(wildcard *.mk)
