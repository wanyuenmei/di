(define Namespace "CHANGE_ME")
(define AdminACL (list "local"))
(label "sshkeys" (githubKey "ejj"))

(label "masters" (machine))
(label "workers" (makeList 3 (machine)))
(label "all-machines" "masters" "workers")
(machineAttribute "all-machines" (provider "AmazonSpot") (size "m4.large"))

// XXX: Once we have lambda this could be simplified with a map and a range
// function.

// XXX: Zookeeper gets confused if you set the local IP address to the label IP
// (instead of 0.0.0.0).  This is likely due to a design flaw in our network
// architecture.  Need to look into it.
(let ((image "quay.io/netsys/zookeeper")) (list
    (label "zoo1" (docker image "1" "0.0.0.0,zoo2.di,zoo3.di"))
    (label "zoo2" (docker image "2" "zoo1.di,0.0.0.0,zoo3.di"))
    (label "zoo3" (docker image "3" "zoo1.di,zoo2.di,0.0.0.0"))))

// XXX: Sigh -- need a much better way to do this.
(let ((zooList (list "zoo1" "zoo2" "zoo3")) (portRange (list 1000 65535)))
  (list
    (connect portRange  "zoo1" zooList)
    (connect portRange  "zoo2" zooList)
    (connect portRange  "zoo3" zooList)))

(placement "exclusive" "zoo1" "zoo2" "zoo3")
