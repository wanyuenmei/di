// Using unique Namespaces will allow multiple Quilt instances to run on the
// same cloud provider account without conflict.
(define Namespace "CHANGE_ME")

// Defines the set of addresses that are allowed to access Quilt VMs.
(define AdminACL (list "local"))

// Supported providers include "Amazon", "Azure", "Google", and "Vagrant".
(define Provider "Amazon")

// We will apply this configuration to each VM.
(define machineCfg (list (provider Provider)
                         (githubKey "ejj"))) // Change Me.

// Declare Master and Worker Machines.
(makeList 1 (machine (role "Master") machineCfg))
(makeList 1 (machine (role "Worker") machineCfg))

// Declare Nginx Docker containers, assigning them the label "web_tier".
(label "web_tier" (list (docker "nginx")))

// Allow http requests from outside world to "web_tier".
(connect 80 "public" "web_tier")
