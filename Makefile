export GO15VENDOREXPERIMENT=1

all: format
	# A Go build bug causes it to behave badly with symlinks.
	cd -P . && go build . && go build -o ./minion/minion ./minion

install:
	cd -P . && go install .

deps:
	cd -P . && glide up --update-vendored

generate:
	for package in `go list ./... | grep -v vendor`; do \
		go generate $$package ; \
	done

format:
	gofmt -w -s .

docker:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build . && \
	docker build -t quay.io/netsys/di . \
	&& cd -P minion && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build . && \
	docker build -t quay.io/netsys/di-minion .

check:
	for package in `go list ./... | grep -v vendor`; do \
		go test $$package ; \
	done

lint:
	cd -P .
	for package in `go list ./... | grep -v vendor`; do \
		go vet $$package ; \
	done
	for package in `go list ./... | grep -v minion/pb | grep -v vendor`; do \
		golint -min_confidence .25 $$package ; \
	done

coverage: db.cov dsl.cov engine.cov cluster.cov join.cov minion/supervisor.cov minion/network.cov minion.cov provider.cov

%.cov:
	go test -coverprofile=$@.out ./$*
	go tool cover -html=$@.out -o $@.html
	rm $@.out

# Include all .mk files so you can have your own local configurations
include $(wildcard *.mk)
