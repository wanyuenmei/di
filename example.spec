// Using unique Namespaces will allow multiple Quilt instances to run on the
// same cloud provider account without conflict.
(define Namespace "CHANGE_ME")

// Defines the set of addresses that are allowed to access Quilt VMs.
(define AdminACL (list "local"))

//We will apply this configuration to each VM.
(define machineCfg
  (list (provider "Amazon")
    (region "us-west-2")
    (size "m3.medium")
    (githubKey "sirspinach")))

(define numMasters 1)
(define numWorkers 1)

//Declare Master and Worker Machines.
(makeList numMasters (machine (role "Master") machineCfg))
(makeList numWorkers (machine (role "Worker") machineCfg))

//Declare Nginx Docker containers, assigning them the label "web_tier".
(label "web_tier" (list (docker "nginx")))

//Allow http requests from outside world to "web_tier".
(connect 80 "public" "web_tier")
