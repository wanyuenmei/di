(import "etcd") // Import etcd.spec

(define nWorker 3)
(etcd.New "etcdexample" nWorker)

// Using unique Namespaces will allow multiple Quilt instances to run on the
// same cloud provider account without conflict.
(define Namespace "CHANGE_ME")

// Defines the set of addresses that are allowed to access Quilt VMs.
(define AdminACL (list "local"))

(let ((cfg (list (provider "Amazon")
  (cpu 2)
  (ram 2)
  (diskSize 32)
  (githubKey "<YOUR_GITHUB_USERNAME>"))))
    (makeList 1 (machine (role "Master") cfg))
    (makeList nWorker (machine (role "Worker") cfg)))
