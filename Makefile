PACKAGES=$(shell GO15VENDOREXPERIMENT=1 go list ./... | grep -v vendor)

all: format
	# A Go build bug causes it to behave badly with symlinks.
	cd -P . && \
	GO15VENDOREXPERIMENT=1 go build . && \
	GO15VENDOREXPERIMENT=1 go build -o ./minion/minion ./minion

install:
	cd -P . && GO15VENDOREXPERIMENT=1 go install .

deps:
	cd -P . && GO15VENDOREXPERIMENT=1 glide up --update-vendored

generate:
	GO15VENDOREXPERIMENT=1 go generate $(PACKAGES)

format:
	gofmt -w -s .

docker: build-linux
	docker build -t quay.io/netsys/di .
	cd -P minion && docker build -t quay.io/netsys/di-minion .
	cd -P di-tester && docker build -t quay.io/netsys/di-tester .

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO15VENDOREXPERIMENT=1 go build .
	cd -P minion && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO15VENDOREXPERIMENT=1 go build .

check:
	GO15VENDOREXPERIMENT=1 go test $(PACKAGES)

lint:
	cd -P . && GO15VENDOREXPERIMENT=1 go vet $(PACKAGES)
	for package in `GO15VENDOREXPERIMENT=1 go list ./... | grep -v minion/pb | grep -v vendor`; do \
		golint -min_confidence .25 $$package ; \
	done

coverage: db.cov dsl.cov engine.cov cluster.cov join.cov minion/supervisor.cov minion/network.cov minion.cov provider.cov

%.cov:
	GO15VENDOREXPERIMENT=1 go test -coverprofile=$@.out ./$*
	GO15VENDOREXPERIMENT=1 go tool cover -html=$@.out -o $@.html
	rm $@.out

# Include all .mk files so you can have your own local configurations
include $(wildcard *.mk)
