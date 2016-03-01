all: format
	# A Go build bug causes it to behave badly with symlinks.
	cd -P . && go build . && go build -o ./minion/minion ./minion

install:
	cd -P . && go install ./...

generate:
	go generate ./...

format:
	gofmt -w -s .

docker:
	cd -P minion && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build . && \
	docker build -t quay.io/netsys/di-minion .

check:
	go test ./...

lint:
	cd -P . && go vet ./...
	for package in `go list ./... | grep -v minion/pb`; do \
	    golint -min_confidence .25 $$package ; \
	done

coverage: db.cov dsl.cov engine.cov cluster.cov join.cov minion/supervisor.cov minion/network.cov minion.cov

%.cov:
	go test -coverprofile=$@.out ./$*
	go tool cover -html=$@.out -o $@.html
	rm $@.out

# Include all .mk files so you can have your own local configurations
include $(wildcard *.mk)
