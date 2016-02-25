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

## Developing the Minion
Whenever you develop code in `minion`, make sure you run your personal minion
image, and not the default DI minion image.
To do that, follow these steps:

1. Create a new empty repository on your favorite registry -
[docker hub](https://hub.docker.com/) or [quay.io](https://quay.io/) for
example.
2. Modify `minionImage` in [util.go](util/util.go) to point to your repo.
3. Modify the repo [Makefile](Makefile) so it builds your image.
4. Create the docker image: `make docker`
   * Since Docker requires certain Linux features, you can't run Docker
   natively on OS X or other non-Linux boxes. A simple workaround is Docker's
   [Docker Quickstart Terminal](https://docs.docker.com/mac/step_one/) which
   provides you with a simple way to set up an appropriate environment.
5. Sign in to your image registry using `docker login`.
6. Push your image: `docker push $YOUR_REPO`. You can consider adding this to
your Makefile as well.

After the above setup, you're good to go - just remember to build and push your
image first, whenever you want to run the `minion` with your latest changes.
