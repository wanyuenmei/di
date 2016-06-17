(import "redis") // Import redis.spec

(define nWorker 3)

// Boot redis with 2 workers and 1 master. AUTH_PASSWORD is used to secure
// the redis connection
(let ((rds (redis.New "redisexample" 2 "<AUTH_PASSWORD>")))
  (redis.Exclusive rds))

// Using unique Namespaces will allow multiple Quilt instances to run on the
// same cloud provider account without conflict.
(define Namespace "CHANGE_ME")

// Defines the set of addresses that are allowed to access Quilt VMs.
(define AdminACL (list "local"))

(let ((cfg (list (provider "Amazon")
                 (cpu 2) (ram 2)
                 (githubKey "<YOUR_GITHUB_USERNAME>"))))
  (makeList 1 (machine (role "Master") cfg))
  (makeList nWorker (machine (role "Worker") cfg)))
