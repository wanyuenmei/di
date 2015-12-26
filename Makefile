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
	cd -P minion && CGO_ENABLED=0 go build . && docker build -t quay.io/netsys/di-minion .

check:
	go test ./...

vet:
	cd -P . && go vet ./...

coverage: db.cov dsl.cov engine.cov cluster.cov minion/supervisor.cov minion.cov

%.cov:
	go test -coverprofile=$@.out ./$*
	go tool cover -html=$@.out -o $@.html
	rm $@.out
