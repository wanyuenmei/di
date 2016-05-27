export GO15VENDOREXPERIMENT=1
PACKAGES=$(shell GO15VENDOREXPERIMENT=1 go list ./... | grep -v vendor)
NOVENDOR=$(shell find . -path ./vendor -prune -o -name '*.go' -print)
REPO = quilt
DOCKER = docker

all:
	cd -P . && \
	go build . && \
	go build -o ./minion/minion ./minion && \
	go build -o ./inspect/inspect ./inspect

install:
	cd -P . && go install . && \
	go install ./inspect

generate:
	go generate $(PACKAGES)

providers:
	python3 scripts/gce-descriptions > provider/gceConstants.go

format:
	gofmt -w -s $(NOVENDOR)

check:
	go test $(PACKAGES)

lint: format
	cd -P . && go vet $(PACKAGES)
	for package in `echo $(PACKAGES) | grep -v minion/pb`; do \
		golint -min_confidence .25 $$package ; \
	done

coverage: db.cov dsl.cov engine.cov cluster.cov join.cov minion/supervisor.cov minion/network.cov minion.cov provider.cov

%.cov:
	go test -coverprofile=$@.out ./$*
	go tool cover -html=$@.out -o $@.html
	rm $@.out

# BUILD
docker-build-all: docker-build-quilt docker-build-tester docker-build-minion docker-build-ovs

docker-build-tester:
	cd -P quilt-tester && ${DOCKER} build -t ${REPO}/tester .

docker-build-minion:
	cd -P minion && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build . \
	 && ${DOCKER} build -t ${REPO}/minion .

docker-build-ovs:
	cd -P ovs && docker build -t ${REPO}/ovs .

# PUSH
#
docker-push-all: docker-push-quilt docker-push-tester docker-push-minion
	# We do not push the OVS container as it's built by the automated
	# docker hub system.

docker-push-tester:
	${DOCKER} push ${REPO}/tester

docker-push-minion:
	${DOCKER} push ${REPO}/minion

docker-push-ovs:
	${DOCKER} push ${REPO}/ovs

# Include all .mk files so you can have your own local configurations
include $(wildcard *.mk)
