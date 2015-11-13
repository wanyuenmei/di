all:
	# A Go build bug causes it to behave badly with symlinks.
	cd -P . && go build
	mkdir -p di-minion/rpc
	cd -P di-minion && protoc di.proto --go_out=plugins=grpc:rpc && go build

install:
	cd -P . && go install
	mkdir -p di-minion/rpc
	cd -P di-minion &&  protoc di.proto --go_out=plugins=grpc:rpc && go install
