# Code Structure
Quilt is structured around a central database (`db`) that stores information about
the current state of the system. This information is used both by the global
controller (Quilt Global) that runs locally on your machine, and by the `minion`
containers on the remote machines.

### Database
Quilt uses the basic `db` database implemented in `db.go`. This database supports
insertions, deletions, transactions, triggers and querying.

The `db` holds the tables defined in `table.go`, and each table is simply a
collection of `row`s. Each `row` is in turn an instance of one of the types
defined in the `db` directory - e.g. `Cluster` or `Machine`. Note that a
`table` holds instances of exactly one type. For instance, in `ClusterTable`,
each `row` is an instance of `Cluster`; in `ConnectionTable`, each `row` is an
instance of `Connection`, and so on. Because of this structure, a given row can
only appear in exactly one table, and the developer therefore performs
insertions, deletions and transactions on the `db`, rather than on specific
tables. Because there is only one possible `table` for any given `row`, this is
safe.

The canonical way to query the database is by calling a `SelectFromX` function
on the `db`. There is a `SelectFromX` function for each type `X` that is stored
in the database. For instance, to query for `Connection`s in the
`ConnectionTable`, one should use `SelectFromConnection`.

## Quilt Global

The first thing that happens when Quilt starts is that your config file is parsed
by `dsl`. `dsl` then puts the connection and container specifications into a
sensible format and forwards them to the `engine`.

The `engine` is responsible for keeping the `db` updated so it always reflects
the desired state of the system. It does so by computing a diff of the config
spec and the current state stored in the database. After identifying the
differences, `engine` determines the least disruptive way to update the
database to the correct state, and then performs these updates. Notice that the
`engine` only updates the database, not the actual remote system - `cluster`
takes care of that.

The `cluster` takes care of making the state of your system equal to the state
of the database. `cluster` continuously checks for updates to the database, and
whenever the state changes, `cluster` boots or terminates VMs in you system to
reflect the changes in the `db`.

Now that VMs are running, the `minion` container will take care of starting the
necessary system containers on its host VM. The `foreman` acts like the middle
man between your locally run Quilt Global, and the `minion` on the VMs. Namely,
the `foreman` configures the `minion`, notifies it of its (the `minion`'s)
role, and passes it the policies from Quilt Global.

All of these steps are done continuously so the config spec, database and
remote system always agree on the state of the system.

## Quilt Remote

As described above, `cluster` is responsible for booting VMs. On boot, each VM
runs docker and a `minion`. The VM is furthermore assigned a role - either
`worker` or `master` - which determines what tasks it will carry out. The
`master` minion is responsible for control related tasks, whereas the `worker`
VMs do "the actual work" - that is, they run containers. When the user
specifies a new container the config file, the scheduler will choose a worker
VM to boot this container on. The `minion` on the chosen VM is then notified,
and will boot the new container on its host. The `minion` is similarly
responsible for tearing down containers on its host VM.

While it is possible to boot multiple `master` VMs, there is only one effective
`master` at any given time. The remaining `master` VMs simply perform as
backups in case the leading `master` fails.

# Development Instructions

The project is written in Go and therefore follows the standard Go
workspaces project style.  The first step is to create a go workspace as
suggested in the [documentation](https://golang.org/doc/code.html).

We currently require go version 1.3 or later.  Ubuntu 15.10 uses this version
by default, so you should just be able to apt-get install golang to get
started.

Since the repository is private, you can't use "go get" to checkout the source
code, so you'll have to do so manually:

    git clone git@github.com:NetSys/quilt.git $GOPATH/src/github.com/NetSys/quilt

Once this is done you can install the AWS API and various other dependencies
automatically:

    go get github.com/NetSys/quilt/...

And finally to build the project run:

    go install github.com/NetSys/quilt

Or alternatively just "go install" if you're in the repo.

## Protobufs
If you change any of the proto files, you'll need to regenerate the protobuf
code.  This requres you to install the protobuf compiler found
[here](https://developers.google.com/protocol-buffers/).  And alls
proto-gen-go.

    go get -u github.com/golang/protobuf/{proto,protoc-gen-go}

To generate the protobufs simply call:

    make generate

## Dependencies
We use [Godep](https://github.com/tools/godep) as dependency vendoring tool. To add a
new dependency, make sure `GO15VENDOREXPERIMENT` is set to 1, then run:

1. `godep restore` to install the package versions specified in `Godeps/Godeps.json` to your `$GOPATH`
2. Run `go get foo/bar`
3. Edit your code to import foo/bar
4. Run `godep save ./...`

## Containers
Some of the functionality that isn't captured in this repo is packaged into
containers that can be found in the following repos:

* [ovs-containers](https://github.com/NetSys/ovs-containers)

## Developing the Minion
Whenever you develop code in `minion`, make sure you run your personal minion
image, and not the default Quilt minion image.  To do that, follow these steps:

1. Create a new empty repository on your favorite registry -
[docker hub](https://hub.docker.com/) for example.
2. Modify `minionImage` in [cloud_config.go](provider/cloud_config.go) to point to your repo.
3. Create a `.mk` file (for example: `local.mk`) to override variables
defined in [Makefile](Makefile). Set `REPO` to your own repository
(for example: `REPO = sample_repo`) inside the `.mk` file you created.
4. Create the docker image: `make docker-build-minion`
   * Since Docker requires certain Linux features, you can't run Docker
   natively on OS X or other non-Linux boxes. A simple workaround is Docker's
   [Docker Quickstart Terminal](https://docs.docker.com/mac/step_one/) which
   provides you with a simple way to set up an appropriate environment.
5. Sign in to your image registry using `docker login`.
6. Push your image: `make docker-push-minion`. There is also a combined
make target called `make docker-minion` which builds and pushes.

After the above setup, you're good to go - just remember to build and push your
image first, whenever you want to run the `minion` with your latest changes.
