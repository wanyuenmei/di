(import "labels")
(import "strings")

(define image "quay.io/netsys/di-wordpress")

(define (hostStr labels)
  (let ((hosts (map labels.Hostname labels)))
    (strings.Join hosts ",")))

(define (makeArgs db memcached)
  (list
    (list "--dbm"
          (hostStr (hmapGet db "master")))
    (if (hmapContains db "slave")
      (list "--repl-mysql"
            (hostStr (hmapGet db "slave"))))
    (if memcached
      (list "--memcached"
            (hostStr memcached)))
    "apache2-foreground"))

(define (link wordpress db memcached)
  (if db
    (let ((dbm (hmapGet db "master"))
          (dbs (hmapGet db "slave")))
      (connect 3306 wordpress dbm)
      (connect 3306 wordpress dbs)))
  (connect 11211 wordpress memcached))

// db: hmap
//   "master": list of db master nodes
//   "slave": list of db slave nodes
// memcached: list of memcached nodes
(define (New prefix n db memcached)
  (let ((args (makeArgs db memcached))
        (wp (makeList n (docker image args)))
        (labelNames (labels.Range prefix n))
        (wordpress (map label labelNames wp)))
    (if (> n 0)
      (progn
        (link wordpress db memcached)
        wordpress))))
