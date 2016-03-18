(define Namespace "Ethan")
(define AdminACL (list "local"))
(label "sshkeys" (githubKey "ejj"))

(label "masters" (machine))
(label "workers" (makeList 3 (machine)))
(label "all-machines" "masters" "workers")
(machineAttribute "all-machines" (provider "AmazonSpot") (size "m4.large"))

// XXX: Zookeeper gets confused if you set the local IP address to the label IP
// (instead of 0.0.0.0).  This is likely due to a design flaw in our network
// architecture.  Need to look into it.
(label "zoo1" (docker "quay.io/netsys/zookeeper" "1" "0.0.0.0,zoo2.di,zoo3.di"))
(label "zoo2" (docker "quay.io/netsys/zookeeper" "2" "zoo1.di,0.0.0.0,zoo3.di"))
(label "zoo3" (docker "quay.io/netsys/zookeeper" "3" "zoo1.di,zoo2.di,0.0.0.0"))
(label "all-zoos" "zoo1" "zoo2" "zoo3")

(placement "exclusive" "all-zoos" "all-zoos")

// XXX: Sigh -- need a much better way to do this.
(connect (list 1000 65535) "zoo1" "zoo1")
(connect (list 1000 65535) "zoo1" "zoo2")
(connect (list 1000 65535) "zoo1" "zoo3")

(connect (list 1000 65535) "zoo2" "zoo1")
(connect (list 1000 65535) "zoo2" "zoo2")
(connect (list 1000 65535) "zoo2" "zoo3")

(connect (list 1000 65535) "zoo3" "zoo1")
(connect (list 1000 65535) "zoo3" "zoo2")
(connect (list 1000 65535) "zoo3" "zoo3")
