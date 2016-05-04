(import "strings")

(define image "quay.io/netsys/di-wordpress")

(define (hostEnv dk env labels)
  (let ((hosts (map labelHost labels)))
    (setEnv dk env (strings.Join hosts ","))))

(define (configure wp db memcached)
  (hostEnv wp "MEMCACHED" memcached)
  (hostEnv wp "DB_HOST" (hmapGet db "master"))
  (hostEnv wp "DB_SLAVES" (hmapGet db "slave")))

// db: hmap
//   "master": list of db master nodes
//   "slave": list of db slave nodes
// memcached: list of memcached nodes
(define (New name n db memcached)
  (let ((dk (makeList n (docker image)))
        (labelNames (strings.Range name n))
        (wordpress (map label labelNames dk)))
    (configure wordpress db memcached)
    (connect 3306 wordpress (hmapGet db "master"))
    (connect 3306 wordpress (hmapGet db "slave"))
    (connect 11211 wordpress memcached)
    wordpress))
