(import "github.com/NetSys/quilt/specs/stdlib/strings")

(define image "quay.io/netsys/di-wordpress")

(define (hostEnv dk env labels)
  (let ((hosts (map labelHost labels)))
    (setEnv dk env (strings.Join hosts ","))))

(define (configure wp db memcd)
  (hostEnv wp "DB_MASTER" (hmapGet db "master"))
  (if (hmapContains db "slave")
    (hostEnv wp "DB_SLAVES" (hmapGet db "slave")))
  (if (> 0 (len memcd))
    (hostEnv wp "MEMCACHED" memcd)))

// db: hmap
//   "master": list of db master nodes
//   "slave": list of db slave nodes
// memcd: list of memcached nodes
(define (New name n db memcd)
  (let ((dk (makeList n (docker image)))
        (labelNames (strings.Range name n))
        (wp (map label labelNames dk)))
    (configure wp db memcd)
    (connect 3306 wp (hmapGet db "master"))
    (connect 3306 wp (hmapGet db "slave"))
    (connect 11211 wp memcd)
    wp))
