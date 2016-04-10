(define Namespace "CHANGE_ME")
(define AdminACL (list "local"))
(label "sshkeys" (githubKey "ejj"))

(label "masters" (machine))
(label "workers" (makeList 3 (machine)))
(label "all-machines" "masters" "workers")
(machineAttribute "all-machines" (provider "AmazonSpot") (size "m4.large"))

// XXX: Once we have lambda this could be simplified with a map and a range
(let ((image "quay.io/netsys/zookeeper")
      (zooHosts "zoo1.di,zoo2.di,zoo3.di"))
     (list (label "zoo1" (docker image "1" zooHosts))
           (label "zoo2" (docker image "2" zooHosts))
           (label "zoo3" (docker image "3" zooHosts))))

(let ((zooList (list "zoo1" "zoo2" "zoo3")) (portRange (list 1000 65535)))
    (connect portRange zooList zooList))

(placement "exclusive" "zoo1" "zoo2" "zoo3")
