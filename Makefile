all:
	# A Go build bug causes it to behave badly with symlinks.
	cd -P . && go build
	cd -P di-minion && go build

install:
	cd -P . && go install
	cd -P di-minion && go install
