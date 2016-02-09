Policy Language Design
======================
Designing languages is hard, so it's a good idea to get started early.
This is a living proposal for the policy language.  It may or may not reflect
the actual implementation at any given point in time.

## Syntax
Currently, the policy language is a simplified Lisp.  Lisp is a good choice at
this early stage because its simplicity will allow rapid development and
experimentation.  Eventually, once the design of the language solidifies, we
will implement a more modern syntax for the language.  The Lisp will still
stick around as an intermediate language, but users won't have to deal with it
directly.

The language will cover the basic things we expect from a lisp including
arithmetic, variable binding, conditionals, etc.  We explicitly will not
support recursion thus guaranteeing that all specifications terminate.

## Atoms
```
(docker <image>)
```
Atoms represent the basic unit of computation instantiated by the policy
language.  Typically these will be containers (or kubernetes pods perhaps), but
they may also represent external services (hostnames/IPs).  They are
instantiated in the policy language by simply declaring them (along with their
basic configuration).

```
(docker ubuntu:15.10) # Boot an Ubuntu 15.10 container.
(makeList 5 (docker ubuntu)) # Boot 5 ubuntu:latest containers.
(host external.org) # Register external.org as a reachable hostname.
```

Atoms can also be used to describe administrative constructs that aren't
directly implemented in the dataplane.  Administrators for example:

```
(user github ejj) # Github user ejj
(user github melvinw) # Github user melvinw
```

As DI supports more functionality, atoms will naturally expand to implement
more concepts.

## Labels
```
(label <name> <member list>)
```
Each atom has associated with it a collection of labels that will be used in
the application data plane.  Labels apply to a set of one or more atoms and
labels -- essentially they're named sets.  Recursion is not allowed.  Labels
may not label themselves.

```
# A database is a postgres container.
(label database (docker postgres))

# These 5 apache containers make up the webTier.
(label webTier (makeList 5 (docker apache))

# A deployment consists of a database, a webTier, and a monitoring system
(label deployment (list database webTier (docker monitor)))
```

The same labelling construction will be used for authentication policy as well.

```
# ejj is a graduate student.
(label grad (user github ejj))

# melvinw is an undergraduate
(label undergrad (user github melvinw)

# Undergraduate and graduate students are admins.
(label admin (list grad undergrad))
```

As DI supports more use cases, the same labeling system will apply naturally.

##### Open Questions
* How do overlapping labels work?  Seems like labels should be lexically scoped
  somehow, but it's not clear how the syntax would work.  Perhaps it should
  work more like the **let** keyword?

## Connect
```
(connect <port> <from> <to>)
```
**connect** explicitly allows communication  between the atoms implementing two
labels.  Atoms implementing the *from* label may initiate network connections
to atoms implementing the *to* label over the specified network *port*.
```
# Allow the public internet to connect to the webTier over port 80
(connect 80 publicWeb webTier)

# Allow the webTier to connect to the database on port 1433
(connect 1433 webTier database)

# Allow members of the database tier to talk to each other over any port
(connect (list 0 65535) database database)
```
##### Service Discovery
The labels used in the **connect** keyword have real meaning in the
application dataplane.  The *to* label will be made available to the *from*
atoms as a hostname on which sockets can be opened.  In the above
example the containers implementing *webTier* may open a socket to the
hostname "database.di" which will connect them to the appropriate database
containers.

##### Load Balancing
The **connect** keyword automatically detects if the *to* label consists of a
single or multiple atoms.  If it's a single atom, it allows direct connections.
However, for connections to multiple atoms, the dataplane will automatically
load balance new connections across all available atoms.

##### Firewalling
By default, atoms in DI cannot communicate with each other due to an implicit
"deny all" firewall.  Communication between atoms must be explicitly permitted
by the **connect** keyword.

## Placement
```
(placement <PLACEMENT_TYPE> <label1> <label2> ... <labelN>)
```

Placement Types:
- `exclusive`: Any instance labeled `label1` will never be placed on the
same host as an instance labeled `label2` and vice-versa. If constraints
can't be satisfied then they won't be scheduled.

```
// A 'webServer' and 'database' will never share a host
(label webServer (docker apache))
(label database (docker mysql))
(placement "exclusive" "webServer" "database")

// A 'dataPipeline' will never share a host with another 'dataPipeline'
(label dataPipeline (docker spark))
(placement "exclusive" "dataPipeline" "dataPipeline")
```

