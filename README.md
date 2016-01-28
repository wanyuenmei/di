[![Build Status](https://travis-ci.com/NetSys/di.svg?token=QspQsur4HQKsDUg6Hynm&branch=master)](https://travis-ci.com/NetSys/di)

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

## Stringer
Some constants rely on the
 [stringer](https://godoc.org/golang.org/x/tools/cmd/stringer) tool, which can
be installed with go get:

        go get golang.org/x/tools/cmd/stringer

That done, simply run the following to update the generated files.

        make generate

## Containers
Some of the functionality that isn't captured in this repo is packaged into
containers that can be found in the following repos:

* [ovs-containers](https://github.com/NetSys/ovs-containers)
* [kubelet](https://github.com/NetSys/di-kubelet)

## The Minion
To work on the minion:

1. Create a new empty repository on your favorite registry:
[docker hub](https://hub.docker.com/) or [quay.io](https://quay.io/) for
example.
2. Modify `minionImage` in [util.go](util/util.go) to point to your repo.
3. Modify the repo [Makefile](Makefile).
4. Create the docker image:

    `make docker`

   * Note that this must be run on a Linux box because the minion will also be running
     on a Linux box.
5. Push the image:

    `docker push $YOUR_REPO`
