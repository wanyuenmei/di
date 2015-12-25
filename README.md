# Development Instructions.

The project is written in Go and therefore follows the standard Go
workspaces project style.  The first step is to create a go workspace as
suggested in the [documentation](https://golang.org/doc/code.html).

We currently require go version 1.3 or later.  Ubuntu 15.10 uses this version
by default, so you should just be able to apt-get install golang to get
started.

Since the repository is private, you can't use "go get" to checkout the source
code, so you'll have to do so manually:

    git clone git@github.com:NetSys/di.git $GOPATH/src/github.com/NetSys/di

Once this is done you can install the AWS API and various other dependencies
automatically:

    go get github.com/NetSys/di/...

And finally to build the project run:

    go install github.com/NetSys/di

Or alternatively just "go install" if you're in the repo.

## Protobufs
If you change any of the proto files, you'll need to regenerate the protobuf
code.  This requres you to install the protobuf compiler found
[here](https://developers.google.com/protocol-buffers/).  And alls
proto-gen-go.

    go get -u github.com/golang/protobuf/{proto,protoc-gen-go}

To generate the protobufs simply call:

        make generate
