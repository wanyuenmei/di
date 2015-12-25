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
	go test -v ./dsl

coverage:
	go test -coverprofile=coverage.out ./dsl
	go tool cover -func coverage.out
	go tool cover -html=coverage.out -o coverage.html
