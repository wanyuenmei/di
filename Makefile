all:
	# A Go build bug causes it to behave badly with symlinks.
	cd -P . && go build

install:
	cd -P . && go install
